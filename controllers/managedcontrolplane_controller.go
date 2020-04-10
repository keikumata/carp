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

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrastructurev1alpha1 "github.com/juan-lee/carp/api/v1alpha1"
	"github.com/juan-lee/carp/internal/kubeadm"
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

func (r *ManagedControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.ManagedControlPlane{}).
		Complete(r)
}

func (r *ManagedControlPlaneReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("managedcontrolplane", req.NamespacedName)

	config := kubeadm.Defaults()
	config.InitConfiguration.LocalAPIEndpoint.AdvertiseAddress = "172.17.0.10"
	config.InitConfiguration.NodeRegistration.Name = "controlplane"
	config.ClusterConfiguration.ControlPlaneEndpoint = "13.66.88.140"

	if result, err := r.reconcileSecrets(req, config); err != nil {
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
	existing := corev1.Pod{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: desired.Name}, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Control Plane pod not found, creating", "name", desired.Name)
			desired.Namespace = req.Namespace

			// TODO(jpang): set owner ref
			if err := r.Create(ctx, desired); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}
