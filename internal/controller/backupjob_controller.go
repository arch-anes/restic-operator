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
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	backupv1 "arch-anes/restic-operator/api/v1"
)

// BackupJobReconciler reconciles a BackupJob object
type BackupJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=backup.arch.anes,resources=backupjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.arch.anes,resources=backupjobs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.arch.anes,resources=backupjobs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the BackupJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.0/pkg/reconcile
func (r *BackupJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the BackupJob instance
	var backupJob backupv1.BackupJob
	if err := r.Get(ctx, req.NamespacedName, &backupJob); err != nil {
		log.Error(err, "unable to fetch BackupJob")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Fetch the referenced Repository
	var repository backupv1.Repository
	if err := r.Get(ctx, types.NamespacedName{Name: backupJob.Spec.RepositoryRef.Name, Namespace: req.Namespace}, &repository); err != nil {
		log.Error(err, "unable to fetch Repository", "repository", backupJob.Spec.RepositoryRef.Name)
		return ctrl.Result{}, err
	}

	// Fetch the referenced Secret
	var secret corev1.Secret
	if err := r.Get(ctx, types.NamespacedName{Name: repository.Spec.SecretRef.Name, Namespace: req.Namespace}, &secret); err != nil {
		log.Error(err, "unable to fetch Secret", "secret", repository.Spec.SecretRef.Name)
		return ctrl.Result{}, err
	}

	// Create or update the CronJob
	cronJob := r.buildCronJob(&backupJob, &repository, &secret)
	if err := ctrl.SetControllerReference(&backupJob, cronJob, r.Scheme); err != nil {
		log.Error(err, "unable to set controller reference")
		return ctrl.Result{}, err
	}

	// Check if the CronJob already exists
	var existingCronJob batchv1.CronJob
	if err := r.Get(ctx, client.ObjectKey{Name: cronJob.Name, Namespace: cronJob.Namespace}, &existingCronJob); err != nil {
		if errors.IsNotFound(err) {
			// Create the CronJob
			log.Info("Creating CronJob", "cronjob", cronJob.Name)
			if err := r.Create(ctx, cronJob); err != nil {
				log.Error(err, "unable to create CronJob")
				return ctrl.Result{}, err
			}
		} else {
			log.Error(err, "unable to fetch CronJob")
			return ctrl.Result{}, err
		}
	} else {
		// Update the CronJob
		log.Info("Updating CronJob", "cronjob", cronJob.Name)
		if err := r.Update(ctx, cronJob); err != nil {
			log.Error(err, "unable to update CronJob")
			return ctrl.Result{}, err
		}
	}

	// Update the BackupJob status
	backupJob.Status.LastBackupTime = &metav1.Time{Time: time.Now()}
	if err := r.Status().Update(ctx, &backupJob); err != nil {
		log.Error(err, "unable to update BackupJob status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *BackupJobReconciler) buildCronJob(backupJob *backupv1.BackupJob, repository *backupv1.Repository, secret *corev1.Secret) *batchv1.CronJob {
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backupJob.Name,
			Namespace: backupJob.Namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule: backupJob.Spec.Schedule,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:    "backup",
									Image:   "restic/restic:latest",
									Command: []string{"restic", "backup", "/data"},
									Env: []corev1.EnvVar{
										{
											Name:  "AWS_ACCESS_KEY_ID",
											Value: string(secret.Data["accessKey"]),
										},
										{
											Name:  "AWS_SECRET_ACCESS_KEY",
											Value: string(secret.Data["secretKey"]),
										},
									},
									VolumeMounts: []corev1.VolumeMount{
										{
											Name:      "data",
											MountPath: "/data",
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								{
									Name: "data",
									VolumeSource: corev1.VolumeSource{
										PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
											ClaimName: backupJob.Spec.Source.PVC,
										},
									},
								},
							},
							RestartPolicy: corev1.RestartPolicyOnFailure,
						},
					},
				},
			},
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1.BackupJob{}).
		Named("backupjob").
		Complete(r)
}
