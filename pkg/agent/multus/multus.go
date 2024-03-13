package multus

// Generate a code which creates a kubernetes Job object with an image
// and uses a selector to only deploy in a node, whose name can be be picked up from the config AgentConfig.NodeName

import (
	"context"
	"fmt"
	"time"

	"github.com/k3s-io/k3s/pkg/daemons/config"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func StartMultusJob(ctx context.Context, nodeConfig *config.Node) {
	imageName, err := fetchImageName(ctx, nodeConfig)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Printf("MANU - This is the imageName: %v\n", imageName)

	job := generateJob(nodeConfig.AgentConfig.NodeName, imageName, 	nodeConfig.AgentConfig.CNIBinDir)

	fmt.Printf("MANU - This is the job: %v\n", job)
	config, err := clientcmd.BuildConfigFromFlags("", nodeConfig.AgentConfig.KubeConfigK3sController)
	if err != nil {
		fmt.Println(err)
	}

	// Create a new clientset
	clientset, _ := kubernetes.NewForConfig(config)
	_, err = clientset.BatchV1().Jobs("kube-system").Create(context.TODO(), &job, metav1.CreateOptions{})
	if err != nil {
		fmt.Println(err)
	}
}

// fetchImageName() is a function that returns the image name by using helm and reading the initContainer of the multus chart
func fetchImageName(ctx context.Context, nodeConfig *config.Node) (string, error) {
	config, err := clientcmd.BuildConfigFromFlags("", nodeConfig.AgentConfig.KubeConfigK3sController)
	if err != nil {
			return "", err
	}

	// Create a new clientset
    clientset, _ := kubernetes.NewForConfig(config)
    daemonset, _ := clientset.AppsV1().DaemonSets("kube-system").Get(context.TODO(), "multus", metav1.GetOptions{})

	var imageName string
	if err := wait.PollImmediateWithContext(ctx, 5*time.Second, 100*time.Second, func(ctx context.Context) (bool, error) {
	    for _, initContainer := range daemonset.Spec.Template.Spec.InitContainers {
    	    if initContainer.Name == "cni-plugins" {
        	    imageName = initContainer.Image
				fmt.Printf("MANU - ImageName found! %v\n", imageName)
				return true, nil
        	}
    	}
		fmt.Printf("MANU - Could not find the multus initContainer image, trying again in 10 seconds\n")
		return false, nil
	}); err != nil {
		return "", fmt.Errorf("time out trying to find the multus initContainer image")
	}

	return imageName, nil
}

func generateJob(nodeName, imageName, cniBinDir string) batchv1.Job {
	hostPathDirectoryOrCreate := corev1.HostPathDirectoryOrCreate
	// Create a new Job object
	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: "multus",
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multus",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "multus",
							Image: imageName,
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/host/opt/cni/bin",
									Name:      "cni-path",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "SKIP_CNI_BINARIES",
									Value: "flannel",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "cni-path",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: cniBinDir,
									Type: &hostPathDirectoryOrCreate,
								},
							},
						},
					},
					NodeSelector: map[string]string{
						"kubernetes.io/hostname": nodeName,
					},
				},
			},
		},
	}

	return job
}