package repository_test

import (
	"os"
	"testing"

	"bitbucket.org/sudosweden/dockyards-backend/pkg/api/v1alpha1"
	"bitbucket.org/sudosweden/dockyards-git/pkg/repository"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReconcileContainerImageRepository(t *testing.T) {
	tt := []struct {
		name     string
		existing *v1alpha1.ContainerImageDeployment
		update   v1alpha1.ContainerImageDeployment
	}{
		{
			name: "test missing existing",
			update: v1alpha1.ContainerImageDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "127bde71-1931-43c2-a133-2ea6b36324b8",
				},
				Spec: v1alpha1.ContainerImageDeploymentSpec{
					Image: "test",
				},
			},
		},
		{
			name: "test image update",
			existing: &v1alpha1.ContainerImageDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "fd47f5f0-2536-4027-9f01-aec13e209829",
				},
				Spec: v1alpha1.ContainerImageDeploymentSpec{
					Image: "test:v1.2.3",
				},
			},
			update: v1alpha1.ContainerImageDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "fd47f5f0-2536-4027-9f01-aec13e209829",
				},
				Spec: v1alpha1.ContainerImageDeploymentSpec{
					Image: "test:v2.3.4",
				},
			},
		},
		{
			name: "test no update",
			existing: &v1alpha1.ContainerImageDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "dd52161a-2ef4-4bb5-bcdc-ca8da00342f2",
				},
				Spec: v1alpha1.ContainerImageDeploymentSpec{
					Image: "test:v1.2.3",
				},
			},
			update: v1alpha1.ContainerImageDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "dd52161a-2ef4-4bb5-bcdc-ca8da00342f2",
				},
				Spec: v1alpha1.ContainerImageDeploymentSpec{
					Image: "test:v1.2.3",
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			dirTemp, err := os.MkdirTemp("", "dockyards-git-")
			if err != nil {
				t.Fatalf("unexpected error creating temporary directory: %s", err)
			}

			r := repository.GitRepository{
				GitProjectRoot: dirTemp,
			}

			ownerDeployment := v1alpha1.Deployment{
				Spec: v1alpha1.DeploymentSpec{
					TargetNamespace: "testing",
				},
			}

			if tc.existing != nil {
				_, err := r.ReconcileContainerImageRepository(tc.existing, &ownerDeployment)
				if err != nil {
					t.Fatalf("error preparing container image repository: %s", err)
				}
			}

			_, err = r.ReconcileContainerImageRepository(&tc.update, &ownerDeployment)
			if err != nil {
				t.Errorf("error reconciling container image repository: %s", err)
			}
		})
	}
}

func TestReconcileKustomizeRepository(t *testing.T) {
	tt := []struct {
		name     string
		existing *v1alpha1.KustomizeDeployment
		update   v1alpha1.KustomizeDeployment
	}{
		{
			name: "test missing existing",
			update: v1alpha1.KustomizeDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "5ffbd5b0-a7c2-4341-b2ec-e18c34e9ec18",
				},
				Spec: v1alpha1.KustomizeDeploymentSpec{
					Kustomize: map[string][]byte{
						"kustomization.yaml": []byte("test"),
					},
				},
			},
		},
		{
			name: "test add file",
			existing: &v1alpha1.KustomizeDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "2ed765fa-cf26-4ec0-86ee-53738489b6af",
				},
				Spec: v1alpha1.KustomizeDeploymentSpec{
					Kustomize: map[string][]byte{
						"kustomization.yaml": []byte("test"),
					},
				},
			},
			update: v1alpha1.KustomizeDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "2ed765fa-cf26-4ec0-86ee-53738489b6af",
				},
				Spec: v1alpha1.KustomizeDeploymentSpec{
					Kustomize: map[string][]byte{
						"kustomization.yaml": []byte("test"),
						"test.yaml":          []byte("yaml"),
					},
				},
			},
		},
		{
			name: "test no change",
			existing: &v1alpha1.KustomizeDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "6dab31e9-9e30-4653-b2bc-a1b576778f4d",
				},
				Spec: v1alpha1.KustomizeDeploymentSpec{
					Kustomize: map[string][]byte{
						"kustomization.yaml": []byte("test"),
					},
				},
			},
			update: v1alpha1.KustomizeDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "6dab31e9-9e30-4653-b2bc-a1b576778f4d",
				},
				Spec: v1alpha1.KustomizeDeploymentSpec{
					Kustomize: map[string][]byte{
						"kustomization.yaml": []byte("test"),
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			dirTemp, err := os.MkdirTemp("", "dockyards-git-")
			if err != nil {
				t.Fatalf("unexpected error creating temporary directory: %s", err)
			}

			r := repository.GitRepository{
				GitProjectRoot: dirTemp,
			}

			if tc.existing != nil {
				_, err = r.ReconcileKustomizeRepository(tc.existing)
				if err != nil {
					t.Errorf("error preparing kustomize deployment repository: %s", err)
				}
			}

			_, err = r.ReconcileKustomizeRepository(&tc.update)
			if err != nil {
				t.Errorf("error reconciling kustomize deployment repository: %s", err)
			}
		})
	}
}
