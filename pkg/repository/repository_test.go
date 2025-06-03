// Copyright 2025 Sudo Sweden AB
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package repository_test

import (
	"os"
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
	dockyardsv1 "github.com/sudoswedenab/dockyards-backend/api/v1alpha3"
	"github.com/sudoswedenab/dockyards-git/pkg/repository"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
