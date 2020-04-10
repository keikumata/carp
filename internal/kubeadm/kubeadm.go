/*
Copyright 2020 Juan-Lee Pang.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package kubeadm

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	"k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta2"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/certs"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/controlplane"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/etcd"
	"k8s.io/kubernetes/cmd/kubeadm/app/phases/kubeconfig"
	etcdutil "k8s.io/kubernetes/cmd/kubeadm/app/util/etcd"
)

type Configuration struct {
	InitConfiguration    v1beta2.InitConfiguration
	ClusterConfiguration v1beta2.ClusterConfiguration
}

func Defaults() *Configuration {
	config := Configuration{}
	scheme.Scheme.Default(&config.InitConfiguration)
	scheme.Scheme.Default(&config.ClusterConfiguration)
	return &config
}

func DefaultCluster() *v1beta2.ClusterConfiguration {
	cc := v1beta2.ClusterConfiguration{}
	scheme.Scheme.Default(&cc)
	return &cc
}

func DefaultInit() *v1beta2.InitConfiguration {
	ic := v1beta2.InitConfiguration{}
	scheme.Scheme.Default(&ic)
	return &ic
}

func (c *Configuration) GenerateSecrets() ([]corev1.Secret, error) {
	tmpdir, err := ioutil.TempDir("", "kubernetes")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpdir)
	certsDir := path.Join(tmpdir, "pki")
	kubeconfigDir := path.Join(tmpdir, "kubeconfig")

	initConfig := kubeadmapi.InitConfiguration{}
	scheme.Scheme.Convert(&c.InitConfiguration, &initConfig, nil)
	scheme.Scheme.Convert(&c.ClusterConfiguration, &initConfig.ClusterConfiguration, nil)
	initConfig.ClusterConfiguration.CertificatesDir = certsDir

	if err := certs.CreatePKIAssets(&initConfig); err != nil {
		return nil, err
	}
	if err := kubeconfig.CreateJoinControlPlaneKubeConfigFiles(kubeconfigDir, &initConfig); err != nil {
		return nil, err
	}

	secrets := []corev1.Secret{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "k8s-certs",
			},
			Data: map[string][]byte{},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "etcd-certs",
			},
			Data: map[string][]byte{},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kubeconfig",
			},
			Data: map[string][]byte{},
		},
	}

	files, err := ioutil.ReadDir(certsDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if !file.IsDir() {
			contents, err := ioutil.ReadFile(fmt.Sprintf("%s/%s", certsDir, file.Name()))
			if err != nil {
				return nil, err
			}
			secrets[0].Data[file.Name()] = contents
		}
		if file.IsDir() && file.Name() == "etcd" {
			etcdFiles, err := ioutil.ReadDir(fmt.Sprintf("%s/etcd", certsDir))
			if err != nil {
				return nil, err
			}
			for _, etcdFile := range etcdFiles {
				contents, err := ioutil.ReadFile(fmt.Sprintf("%s/etcd/%s", certsDir, etcdFile.Name()))
				if err != nil {
					return nil, err
				}
				secrets[1].Data[etcdFile.Name()] = contents
			}
		}
	}

	kubeconfigs, err := ioutil.ReadDir(kubeconfigDir)
	if err != nil {
		return nil, err
	}

	for _, file := range kubeconfigs {
		if file.IsDir() {
			continue
		}
		contents, err := ioutil.ReadFile(fmt.Sprintf("%s/%s", kubeconfigDir, file.Name()))
		if err != nil {
			return nil, err
		}
		secrets[2].Data[file.Name()] = contents
	}
	return secrets, nil
}

func (c *Configuration) ControlPlanePodSpec() *corev1.Pod {
	initConfig := kubeadmapi.InitConfiguration{}
	scheme.Scheme.Convert(&c.InitConfiguration, &initConfig, nil)
	scheme.Scheme.Convert(&c.ClusterConfiguration, &initConfig.ClusterConfiguration, nil)
	pods := controlplane.GetStaticPodSpecs(&initConfig.ClusterConfiguration, &initConfig.LocalAPIEndpoint)

	combined := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "controlplane",
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "k8s-certs",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "k8s-certs",
						},
					},
				},
				{
					Name: "etcd-certs",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "etcd-certs",
						},
					},
				},
				{
					Name: "etcd-data",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "flexvolume-dir",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "kubeconfig",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: "kubeconfig",
						},
					},
				},
				{
					Name: "ca-certs",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/etc/ssl/certs",
							Type: hostPathTypePtr(corev1.HostPathDirectoryOrCreate),
						},
					},
				},
			},
		},
	}

	for _, pod := range pods {
		for n := range pod.Spec.Containers {
			if pod.Spec.Containers[n].LivenessProbe != nil && pod.Spec.Containers[n].LivenessProbe.HTTPGet != nil {
				// Substitute 127.0.0.1 with empty string so liveness will use pod ip instead.
				pod.Spec.Containers[n].LivenessProbe.HTTPGet.Host = ""
			}
			for i := range pod.Spec.Containers[n].VolumeMounts {
				if pod.Spec.Containers[n].VolumeMounts[i].Name == "kubeconfig" {
					pod.Spec.Containers[n].VolumeMounts[i].MountPath = "/etc/kubernetes"
				}
			}
		}
		combined.Spec.Containers = append(combined.Spec.Containers, pod.Spec.Containers...)
	}

	etcdPod := etcd.GetEtcdPodSpec(&initConfig.ClusterConfiguration, &initConfig.LocalAPIEndpoint, "controlplane", []etcdutil.Member{})
	for n := range etcdPod.Spec.Containers {
		if etcdPod.Spec.Containers[n].LivenessProbe != nil && etcdPod.Spec.Containers[n].LivenessProbe.HTTPGet != nil {
			// Substitute 127.0.0.1 with empty string so liveness will use etcdPod ip instead.
			etcdPod.Spec.Containers[n].LivenessProbe.HTTPGet.Host = ""
		}
	}
	combined.Spec.Containers = append(combined.Spec.Containers, etcdPod.Spec.Containers...)
	return &combined
}

func hostPathTypePtr(h corev1.HostPathType) *corev1.HostPathType {
	return &h
}
