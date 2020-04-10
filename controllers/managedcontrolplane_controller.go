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

// nolint: dupl
package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrastructurev1alpha1 "github.com/juan-lee/carp/api/v1alpha1"
	"github.com/juan-lee/carp/internal/kubeadm"
	"github.com/juan-lee/carp/internal/tunnel"
)

// ManagedControlPlaneReconciler reconciles a ManagedControlPlane object
type ManagedControlPlaneReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=managedcontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=managedcontrolplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete

func (r *ManagedControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.ManagedControlPlane{}).
		Complete(r)
}

func (r *ManagedControlPlaneReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	log := r.Log.WithValues("managedcontrolplane", req.NamespacedName)

	config := kubeadm.Defaults()
	config.InitConfiguration.LocalAPIEndpoint.AdvertiseAddress = "172.17.0.10"
	config.InitConfiguration.NodeRegistration.Name = "controlplane"
	config.ClusterConfiguration.KubernetesVersion = "v1.18.0"
	config.ClusterConfiguration.Networking.ServiceSubnet = "172.18.0.0/12"

	if result, err := r.reconcileTunnelService(req, config); err != nil {
		return result, err
	}

	if result, err := r.reconcileControlPlaneService(req, config); err != nil {
		return result, err
	}

	if config.ClusterConfiguration.ControlPlaneEndpoint == "" {
		log.Info("ControlPlane ExternalIP missing, requeuing...")
		return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 30}, nil
	}

	if result, err := r.reconcileSecrets(req, config); err != nil {
		return result, err
	}

	if result, err := r.reconcileTunnelServer(req, config); err != nil {
		return result, err
	}

	if result, err := r.reconcileControlPlane(req, config); err != nil {
		return result, err
	}

	return ctrl.Result{}, nil
}

func (r *ManagedControlPlaneReconciler) reconcileSecrets(req ctrl.Request, config *kubeadm.Configuration) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("managedcontrolplane", req.NamespacedName)

	secrets, err := config.GenerateSecrets()
	if err != nil {
		return ctrl.Result{}, err
	}
	tunnelSecrets, err := tunnel.Secrets()
	if err != nil {
		return ctrl.Result{}, err
	}
	secrets = append(secrets, tunnelSecrets...)

	for _, secret := range secrets {
		secret.Namespace = req.Namespace
		existing := corev1.Secret{}
		log.Info("Reconciling secret", "name", secret.Name)
		if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: secret.Name}, &existing); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Secret not found, creating", "name", secret.Name)
				secret.Namespace = req.Namespace

				// TODO(jpang): set owner ref
				if err := r.Create(ctx, &secret); err != nil {
					return ctrl.Result{}, err
				}
				continue
			}
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *ManagedControlPlaneReconciler) reconcileControlPlane(req ctrl.Request, config *kubeadm.Configuration) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("managedcontrolplane", req.NamespacedName)

	desired := config.ControlPlanePodSpec()
	desired.Namespace = req.Namespace
	desired.Spec.Template.Spec.Containers = append(desired.Spec.Template.Spec.Containers, tunnel.ClientPodSpec().Spec.Containers...)
	existing := appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: desired.Name}, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Control Plane pod not found, creating", "name", desired.Name)

			// TODO(jpang): set owner ref
			if err := r.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("Control Plane pod found, updating", "name", desired.Name)
	if err := r.Update(ctx, desired); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ManagedControlPlaneReconciler) reconcileTunnelServer(req ctrl.Request, config *kubeadm.Configuration) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("managedcontrolplane", req.NamespacedName)

	desired := tunnel.ServerPodSpec()
	desired.Namespace = req.Namespace
	existing := appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: desired.Name}, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Tunnel Server pod not found, creating", "name", desired.Name)

			// TODO(jpang): set owner ref
			if err := r.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log.Info("Tunnel Server pod found, updating", "name", desired.Name)
	if err := r.Update(ctx, desired); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *ManagedControlPlaneReconciler) reconcileTunnelService(req ctrl.Request, config *kubeadm.Configuration) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("managedcontrolplane", req.NamespacedName)

	desired := tunnel.ServerServiceSpec()
	desired.Namespace = req.Namespace
	existing := corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: desired.Name}, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Tunnel Server service not found, creating", "name", desired.Name)

			// TODO(jpang): set owner ref
			if err := r.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// log.Info("Tunnel Server pod found, updating", "name", desired.Name)
	// if err := r.Update(ctx, desired); err != nil {
	// 	return ctrl.Result{}, err
	// }
	return ctrl.Result{}, nil
}

func (r *ManagedControlPlaneReconciler) reconcileControlPlaneService(req ctrl.Request, config *kubeadm.Configuration) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("managedcontrolplane", req.NamespacedName)

	desired := kubeadm.ControlPlaneServiceSpec()
	desired.Namespace = req.Namespace
	existing := corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: desired.Name}, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Control Plane service not found, creating", "name", desired.Name)

			// TODO(jpang): set owner ref
			if err := r.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if len(existing.Status.LoadBalancer.Ingress) > 0 {
		config.ClusterConfiguration.ControlPlaneEndpoint = existing.Status.LoadBalancer.Ingress[0].IP
	}

	// log.Info("Control Plane pod found, updating", "name", desired.Name)
	// if err := r.Update(ctx, desired); err != nil {
	// 	return ctrl.Result{}, err
	// }
	return ctrl.Result{}, nil
}
