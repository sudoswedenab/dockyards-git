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

package repository

import (
	"errors"
	"log/slog"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/filesystem"
	dockyardsv1 "github.com/sudoswedenab/dockyards-backend/api/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Repository interface {
	ReconcileWorktree(*dockyardsv1.Worktree) (*plumbing.Reference, *url.URL, error)
}

var (
	ErrUnknownDeploymentType          = errors.New("unsupported deployment type")
	ErrDeploymentNameEmpty            = errors.New("deployment name must not be empty")
	ErrDeploymentImageEmpty           = errors.New("deployment image must not be empty")
	ErrDeploymentKustomizationMissing = errors.New("no kustomization.yaml file provided")
	ErrDeploymentUIDEmpty             = errors.New("deployment uid must not be empty")
	ErrWorkloadUIDEmpty               = errors.New("workload uid must not be empty")
)

type GitRepository struct {
	GitProjectRoot string
	Logger         *slog.Logger
	Hostname       string
}

func isNotFound(err error) bool {
	return errors.Is(err, plumbing.ErrReferenceNotFound)
}

func ignoreNotFound(err error) error {
	if isNotFound(err) {
		return nil
	}

	return err
}

func isNotExists(err error) bool {
	return errors.Is(err, git.ErrRepositoryNotExists)
}

func ignoreNotExists(err error) error {
	if isNotExists(err) {
		return nil
	}

	return err
}

func (r *GitRepository) OpenOrInitRepository(repoPath string, worktree billy.Filesystem) (*git.Repository, error) {
	fs := osfs.New(repoPath)
	storage := filesystem.NewStorage(fs, cache.NewObjectLRUDefault())

	repo, err := git.Open(storage, worktree)
	if ignoreNotExists(err) != nil {
		return nil, err
	}

	if isNotExists(err) {
		initOptions := git.InitOptions{
			DefaultBranch: plumbing.Main,
		}

		_, err := git.InitWithOptions(storage, nil, initOptions)
		if err != nil {
			return nil, err
		}

		repo, err = git.Open(storage, worktree)
		if err != nil {
			return nil, err
		}
	}

	return repo, nil
}

func (r *GitRepository) GetRepositoryURL(repoPath string) *url.URL {
	p := strings.TrimPrefix(repoPath, r.GitProjectRoot)

	u := url.URL{
		Scheme: "http",
		Host:   r.Hostname,
		Path:   p,
	}

	return &u
}

func (r *GitRepository) ReconcileWorktree(worktree *dockyardsv1.Worktree) (*plumbing.Reference, *url.URL, error) {
	if string(worktree.UID) == "" {
		return nil, nil, ErrWorkloadUIDEmpty
	}

	repoPath := path.Join(r.GitProjectRoot, "worktrees", string(worktree.UID))

	mfs := memfs.New()

	repo, err := r.OpenOrInitRepository(repoPath, mfs)
	if err != nil {
		return nil, nil, err
	}

	w, err := repo.Worktree()
	if err != nil {
		return nil, nil, err
	}

	for filename, contents := range worktree.Spec.Files {
		file, err := mfs.Create(filename)
		if err != nil {
			return nil, nil, err
		}

		_, err = file.Write(contents)
		if err != nil {
			return nil, nil, err
		}
	}

	status, err := w.Status()
	if err != nil {
		return nil, nil, err
	}

	reference, err := repo.Head()
	if ignoreNotFound(err) != nil {
		return nil, nil, err
	}

	if status.IsClean() {
		return reference, r.GetRepositoryURL(repoPath), nil
	}

	for file := range status {
		_, err := w.Add(file)
		if err != nil {
			return nil, nil, err
		}
	}

	msg := "Update worktree files"
	if reference == nil {
		msg = "Add worktree files"
	}

	_, err = w.Commit(msg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  "dockyards-git",
			Email: "git@dockyards.io",
			When:  time.Now(),
		},
	})
	if err != nil {
		return nil, nil, err
	}

	reference, err = repo.Head()
	if ignoreNotFound(err) != nil {
		return nil, nil, err
	}

	return reference, r.GetRepositoryURL(repoPath), nil
}

func (r *GitRepository) DeleteRepository(object client.Object) error {
	uid := string(object.GetUID())
	if uid == "" {
		return ErrDeploymentUIDEmpty
	}

	repoPath := path.Join(r.GitProjectRoot, "deployments", string(object.GetUID()))

	_, err := git.PlainOpen(repoPath)
	if ignoreNotExists(err) != nil {
		return err
	}

	if isNotExists(err) {
		r.Logger.Warn("delete non existing repository", "path", repoPath)

		return nil
	}

	err = os.RemoveAll(repoPath)
	if err != nil {
		return err
	}

	return nil
}

var _ Repository = &GitRepository{}
