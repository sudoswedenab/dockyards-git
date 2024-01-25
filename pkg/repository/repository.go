package repository

import (
	"errors"
	"log/slog"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	"bitbucket.org/sudosweden/dockyards-backend/pkg/api/v1alpha1"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/filesystem"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

var (
	ErrUnknownDeploymentType          = errors.New("unsupported deployment type")
	ErrDeploymentNameEmpty            = errors.New("deployment name must not be empty")
	ErrDeploymentImageEmpty           = errors.New("deployment image must not be empty")
	ErrDeploymentKustomizationMissing = errors.New("no kustomization.yaml file provided")
	ErrDeploymentUIDEmpty             = errors.New("deployment uid must not be empty")
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

func createContainerImageDeployment(containerImageDeployment *v1alpha1.ContainerImageDeployment) (*appsv1.Deployment, error) {
	containerPort := int32(80)
	if containerImageDeployment.Spec.Port != 0 {
		containerPort = containerImageDeployment.Spec.Port
	}

	containerPorts := []corev1.ContainerPort{
		{
			Name:          "http",
			Protocol:      corev1.ProtocolTCP,
			ContainerPort: containerPort,
		},
	}

	d := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: containerImageDeployment.Name,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app.kubernetes.io/name": containerImageDeployment.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/name": containerImageDeployment.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            containerImageDeployment.Name,
							Image:           containerImageDeployment.Spec.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports:           containerPorts,
						},
					},
				},
			},
		},
	}

	return &d, nil
}

func createContainerImageService(containerImageDeployment *v1alpha1.ContainerImageDeployment) (*corev1.Service, error) {
	service := corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: containerImageDeployment.Name,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromString("http"),
					Protocol:   corev1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app.kubernetes.io/name": containerImageDeployment.Name,
			},
		},
	}

	return &service, nil
}

func createDeploymentNamespace(deployment *v1alpha1.Deployment) (*corev1.Namespace, error) {
	namespace := corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: deployment.Spec.TargetNamespace,
		},
	}

	return &namespace, nil
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

func (r *GitRepository) GetRepositoryURL(repoPath string) string {
	p := strings.TrimPrefix(repoPath, r.GitProjectRoot)

	u := url.URL{
		Scheme: "http",
		Host:   r.Hostname,
		Path:   p,
	}

	return u.String()
}

func (r *GitRepository) ReconcileContainerImageRepository(containerImageDeployment *v1alpha1.ContainerImageDeployment, ownerDeployment *v1alpha1.Deployment) (string, error) {
	if string(containerImageDeployment.UID) == "" {
		return "", ErrDeploymentUIDEmpty
	}

	namespace, err := createDeploymentNamespace(ownerDeployment)
	if err != nil {
		return "", err
	}

	namespaceYAML, err := yaml.Marshal(namespace)
	if err != nil {
		return "", err
	}

	appsv1Deployment, err := createContainerImageDeployment(containerImageDeployment)
	if err != nil {
		return "", err
	}

	deploymentYAML, err := yaml.Marshal(appsv1Deployment)
	if err != nil {
		return "", err
	}

	service, err := createContainerImageService(containerImageDeployment)
	if err != nil {
		return "", err
	}

	serviceYAML, err := yaml.Marshal(service)
	if err != nil {
		return "", err
	}

	repoPath := path.Join(r.GitProjectRoot, "deployments", string(containerImageDeployment.UID))

	mfs := memfs.New()

	repo, err := r.OpenOrInitRepository(repoPath, mfs)
	if err != nil {
		return "", err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return "", err
	}

	file, err := mfs.Create("namespace.yaml")
	if err != nil {
		return "", err
	}

	_, err = file.Write(namespaceYAML)
	if err != nil {
		return "", err
	}

	file, err = mfs.Create("deployment.yaml")
	if err != nil {
		return "", err
	}

	_, err = file.Write(deploymentYAML)
	if err != nil {
		return "", err
	}

	file.Close()

	file, err = mfs.Create("service.yaml")
	if err != nil {
		return "", err
	}

	_, err = file.Write(serviceYAML)
	if err != nil {
		return "", err
	}

	file.Close()

	status, err := worktree.Status()
	if err != nil {
		return "", err
	}

	if !status.IsClean() {
		for file := range status {
			_, err := worktree.Add(file)
			if err != nil {
				return "", err
			}
		}

		_, err := repo.Head()
		if ignoreNotFound(err) != nil {
			return "", err
		}

		msg := "Update container image manifests"
		if isNotFound(err) {
			msg = "Add container image manifests"
		}

		_, err = worktree.Commit(msg, &git.CommitOptions{
			Author: &object.Signature{
				Name:  "dockyards-git",
				Email: "git@dockyards.io",
				When:  time.Now(),
			},
		})
		if err != nil {
			return "", err
		}
	}

	repositoryURL := r.GetRepositoryURL(repoPath)
	return repositoryURL, nil
}

func (r *GitRepository) ReconcileKustomizeRepository(kustomizeDeployment *v1alpha1.KustomizeDeployment) (string, error) {
	if string(kustomizeDeployment.UID) == "" {
		return "", ErrDeploymentUIDEmpty
	}

	kustomize := kustomizeDeployment.Spec.Kustomize

	_, hasKustomization := kustomize["kustomization.yaml"]
	if !hasKustomization {
		return "", ErrDeploymentKustomizationMissing
	}

	repoPath := path.Join(r.GitProjectRoot, "deployments", string(kustomizeDeployment.UID))

	mfs := memfs.New()

	repo, err := r.OpenOrInitRepository(repoPath, mfs)
	if err != nil {
		return "", err
	}

	w, err := repo.Worktree()
	if err != nil {
		return "", err
	}

	for filename, content := range kustomizeDeployment.Spec.Kustomize {
		file, err := mfs.Create(filename)
		if err != nil {
			return "", err
		}

		_, err = file.Write(content)
		if err != nil {
			return "", err
		}
	}

	status, err := w.Status()
	if err != nil {
		return "", err
	}

	if !status.IsClean() {
		for file := range status {
			_, err := w.Add(file)
			if err != nil {
				return "", err
			}
		}

		_, err := repo.Head()
		if ignoreNotFound(err) != nil {
			return "", err
		}

		msg := "Update kustomize manifests"
		if isNotFound(err) {
			msg = "Add kustomize manifests"
		}

		_, err = w.Commit(msg, &git.CommitOptions{
			Author: &object.Signature{
				Name:  "dockyards-git",
				Email: "git@dockyards.io",
				When:  time.Now(),
			},
		})
		if err != nil {
			return "", err
		}
	}

	repositoryURL := r.GetRepositoryURL(repoPath)
	return repositoryURL, nil
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
