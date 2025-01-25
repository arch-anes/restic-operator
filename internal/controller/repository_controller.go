/*
Copyright 2025.

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

package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	backupv1 "arch-anes/restic-operator/api/v1"

	corev1 "k8s.io/api/core/v1"
)

// RepositoryReconciler reconciles a Repository object
type RepositoryReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=backup.arch.anes,resources=repositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.arch.anes,resources=repositories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.arch.anes,resources=repositories/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Repository object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *RepositoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Repository instance
	var repository backupv1.Repository
	if err := r.Get(ctx, req.NamespacedName, &repository); err != nil {
		log.Error(err, "unable to fetch Repository")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Fetch the referenced Secret
	secretName := repository.Spec.SecretRef.Name
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: secretName, Namespace: req.Namespace}, &secret); err != nil {
		log.Error(err, "unable to fetch Secret", "secret", secretName)
		return ctrl.Result{}, err
	}

	// Use the Secret data
	accessKey := string(secret.Data["accessKey"])
	secretKey := string(secret.Data["secretKey"])
	log.Info("Retrieved credentials", "accessKey", accessKey, "secretKey", secretKey)

	// TODO: extra reconciliation logic here...

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1.Repository{}).
		Named("repository").
		Complete(r)
}
