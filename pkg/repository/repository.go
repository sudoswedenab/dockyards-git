package repository

import (
	"errors"
	"path"
	"strings"
	"time"

	"bitbucket.org/sudosweden/dockyards-backend/pkg/api/v1alpha1"
	// "github.com/containers/image/v5/docker/reference"
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
	"sigs.k8s.io/yaml"
)

var (
	ErrUnknownDeploymentType          = errors.New("unsupported deployment type")
	ErrDeploymentNameEmpty            = errors.New("deployment name must not be empty")
	ErrDeploymentImageEmpty           = errors.New("deployment image must not be empty")
	ErrDeploymentKustomizationMissing = errors.New("no kustomization.yaml file provided")
)

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

func CreateContainerImageRepository(containerImageDeployment *v1alpha1.ContainerImageDeployment, gitProjectRoot string) (string, error) {
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

	repoPath := path.Join(gitProjectRoot, "deployments", string(containerImageDeployment.UID))

	fs := osfs.New(repoPath)
	storage := filesystem.NewStorage(fs, cache.NewObjectLRUDefault())
	initOptions := git.InitOptions{
		DefaultBranch: plumbing.Main,
	}
	mfs := memfs.New()

	repo, err := git.InitWithOptions(storage, mfs, initOptions)
	if err != nil {
		return "", err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return "", err
	}

	file, err := mfs.Create("deployment.yaml")
	if err != nil {
		return "", err
	}

	_, err = file.Write(deploymentYAML)
	if err != nil {
		return "", err
	}

	file.Close()
	worktree.Add("deployment.yaml")

	file, err = mfs.Create("service.yaml")
	if err != nil {
		return "", err
	}

	_, err = file.Write(serviceYAML)
	if err != nil {
		return "", err
	}

	file.Close()
	worktree.Add("service.yaml")

	_, err = worktree.Commit("Add deployment", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "dockyards",
			Email: "git@dockyards.io",
			When:  time.Now(),
		},
	})

	if err != nil {
		return "", err
	}

	repoPath = strings.TrimPrefix(repoPath, gitProjectRoot)
	return repoPath, nil
}

func CreateKustomizeRepository(kustomizeDeployment *v1alpha1.KustomizeDeployment, gitProjectRoot string) (string, error) {
	kustomize := kustomizeDeployment.Spec.Kustomize

	_, hasKustomization := kustomize["kustomization.yaml"]
	if !hasKustomization {
		return "", ErrDeploymentKustomizationMissing
	}

	repoPath := path.Join(gitProjectRoot, "deployments", string(kustomizeDeployment.UID))

	fs := osfs.New(repoPath)
	storage := filesystem.NewStorage(fs, cache.NewObjectLRUDefault())
	initOptions := git.InitOptions{
		DefaultBranch: plumbing.Main,
	}
	mfs := memfs.New()

	repo, err := git.InitWithOptions(storage, mfs, initOptions)
	if err != nil {
		return "", err
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return "", err
	}

	for filename, content := range kustomize {
		file, err := mfs.Create(filename)
		if err != nil {
			return "", err
		}

		_, err = file.Write(content)
		if err != nil {
			return "", err
		}

		file.Close()

		worktree.Add(filename)
	}

	_, err = worktree.Commit("Add kustomize", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "dockyards",
			Email: "git@dockyards.io",
			When:  time.Now(),
		},
	})

	if err != nil {
		return "", err
	}

	repoPath = strings.TrimPrefix(repoPath, gitProjectRoot)
	return repoPath, nil
}
