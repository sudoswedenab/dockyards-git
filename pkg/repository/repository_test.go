package repository_test

import (
	"os"
	"testing"

	dockyardsv1 "bitbucket.org/sudosweden/dockyards-backend/pkg/api/v1alpha3"
	"bitbucket.org/sudosweden/dockyards-git/pkg/repository"
	"github.com/go-git/go-git/v5/plumbing"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReconcileContainerImageRepository(t *testing.T) {
	tt := []struct {
		name       string
		existing   *dockyardsv1.ContainerImageDeployment
		update     dockyardsv1.ContainerImageDeployment
		credential *corev1.Secret
	}{
		{
			name: "test missing existing",
			update: dockyardsv1.ContainerImageDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "127bde71-1931-43c2-a133-2ea6b36324b8",
				},
				Spec: dockyardsv1.ContainerImageDeploymentSpec{
					Image: "test",
				},
			},
		},
		{
			name: "test image update",
			existing: &dockyardsv1.ContainerImageDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "fd47f5f0-2536-4027-9f01-aec13e209829",
				},
				Spec: dockyardsv1.ContainerImageDeploymentSpec{
					Image: "test:v1.2.3",
				},
			},
			update: dockyardsv1.ContainerImageDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "fd47f5f0-2536-4027-9f01-aec13e209829",
				},
				Spec: dockyardsv1.ContainerImageDeploymentSpec{
					Image: "test:v2.3.4",
				},
			},
		},
		{
			name: "test no update",
			existing: &dockyardsv1.ContainerImageDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "dd52161a-2ef4-4bb5-bcdc-ca8da00342f2",
				},
				Spec: dockyardsv1.ContainerImageDeploymentSpec{
					Image: "test:v1.2.3",
				},
			},
			update: dockyardsv1.ContainerImageDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "dd52161a-2ef4-4bb5-bcdc-ca8da00342f2",
				},
				Spec: dockyardsv1.ContainerImageDeploymentSpec{
					Image: "test:v1.2.3",
				},
			},
		},
		{
			name: "test with credential",
			update: dockyardsv1.ContainerImageDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "43502059-2e2e-4e9f-bba8-a23b354e2fb9",
				},
				Spec: dockyardsv1.ContainerImageDeploymentSpec{
					Image: "test:v1.2.3",
				},
			},
			credential: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
				},
				Data: map[string][]byte{
					"secret": []byte("test"),
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

			ownerDeployment := dockyardsv1.Deployment{
				Spec: dockyardsv1.DeploymentSpec{
					TargetNamespace: "testing",
				},
			}

			if tc.existing != nil {
				_, err := r.ReconcileContainerImageRepository(tc.existing, &ownerDeployment, tc.credential)
				if err != nil {
					t.Fatalf("error preparing container image repository: %s", err)
				}
			}

			_, err = r.ReconcileContainerImageRepository(&tc.update, &ownerDeployment, tc.credential)
			if err != nil {
				t.Errorf("error reconciling container image repository: %s", err)
			}
		})
	}
}

func TestReconcileKustomizeRepository(t *testing.T) {
	tt := []struct {
		name     string
		existing *dockyardsv1.KustomizeDeployment
		update   dockyardsv1.KustomizeDeployment
	}{
		{
			name: "test missing existing",
			update: dockyardsv1.KustomizeDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "5ffbd5b0-a7c2-4341-b2ec-e18c34e9ec18",
				},
				Spec: dockyardsv1.KustomizeDeploymentSpec{
					Kustomize: map[string][]byte{
						"kustomization.yaml": []byte("test"),
					},
				},
			},
		},
		{
			name: "test add file",
			existing: &dockyardsv1.KustomizeDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "2ed765fa-cf26-4ec0-86ee-53738489b6af",
				},
				Spec: dockyardsv1.KustomizeDeploymentSpec{
					Kustomize: map[string][]byte{
						"kustomization.yaml": []byte("test"),
					},
				},
			},
			update: dockyardsv1.KustomizeDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "2ed765fa-cf26-4ec0-86ee-53738489b6af",
				},
				Spec: dockyardsv1.KustomizeDeploymentSpec{
					Kustomize: map[string][]byte{
						"kustomization.yaml": []byte("test"),
						"test.yaml":          []byte("yaml"),
					},
				},
			},
		},
		{
			name: "test no change",
			existing: &dockyardsv1.KustomizeDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "6dab31e9-9e30-4653-b2bc-a1b576778f4d",
				},
				Spec: dockyardsv1.KustomizeDeploymentSpec{
					Kustomize: map[string][]byte{
						"kustomization.yaml": []byte("test"),
					},
				},
			},
			update: dockyardsv1.KustomizeDeployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "6dab31e9-9e30-4653-b2bc-a1b576778f4d",
				},
				Spec: dockyardsv1.KustomizeDeploymentSpec{
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

func TestReconcileWorktree(t *testing.T) {
	tt := []struct {
		name     string
		worktree dockyardsv1.Worktree
	}{
		{
			name: "test single file",
			worktree: dockyardsv1.Worktree{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "502b3517-c21c-4da4-97a5-7b7c49fdb380",
				},
				Spec: dockyardsv1.WorktreeSpec{
					Files: map[string][]byte{
						"test": []byte("qwfp"),
					},
				},
			},
		},
		{
			name: "test nested file",
			worktree: dockyardsv1.Worktree{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "testing",
					UID:       "502b3517-c21c-4da4-97a5-7b7c49fdb380",
				},
				Spec: dockyardsv1.WorktreeSpec{
					Files: map[string][]byte{
						"test/nested/file": []byte("qwfp"),
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			dirTemp, err := os.MkdirTemp("", "worktree-")
			if err != nil {
				t.Fatal(err)
			}

			r := repository.GitRepository{
				GitProjectRoot: dirTemp,
				Hostname:       "localhost",
			}

			reference, url, err := r.ReconcileWorktree(&tc.worktree)
			if err != nil {
				t.Fatal(err)
			}

			if reference.Name() != plumbing.Main {
				t.Errorf("expected reference name %s, got %s", plumbing.Main, reference.Name())
			}

			expectedURL := "http://" + r.Hostname + "/worktrees/" + string(tc.worktree.UID)

			if url.String() != expectedURL {
				t.Errorf("expected url %s, got %s", expectedURL, url.String())
			}

		})
	}
}
