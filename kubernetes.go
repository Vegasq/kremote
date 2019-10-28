package main
import (
	"bytes"
	"fmt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"io/ioutil"
	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	exex "k8s.io/kubectl/pkg/cmd/exec"
)


func getConfig(kubeconfig string) (*restclient.Config) {
	config, confErr := clientcmd.BuildConfigFromFlags("", kubeconfig)

	if confErr != nil {
		logrus.Fatalln("Can't read config: %s.\n", confErr)
	}

	config.APIPath = "/api"
	var Unversioned = schema.GroupVersion{Group: "", Version: "v1"}
	config.GroupVersion = &Unversioned
	config.NegotiatedSerializer = scheme.Codecs

	return config
}

func getClientSet(config *restclient.Config) *kubernetes.Clientset {
	if config == nil {
		logrus.Errorf("Empty config: %s", config)
		return &kubernetes.Clientset{}
	}

	var cliSetErr error
	clientSet, cliSetErr := kubernetes.NewForConfig(config)

	if cliSetErr != nil {
		logrus.Errorf("Can't create clientSet: %s", cliSetErr)
	}
	return clientSet
}

func (Kubik) newStreamOptions(pod coreV1.Pod) (exex.StreamOptions, *bytes.Buffer) {
	in := &bytes.Buffer{}
	out := &bytes.Buffer{}

	errOut := &bytes.Buffer{}

	stream := genericclioptions.IOStreams{
		In:     in,
		Out:    out,
		ErrOut: errOut,
	}

	so := exex.StreamOptions{
		PodName:   pod.Name,
		Namespace: pod.Namespace,
		IOStreams: stream,
		Stdin:     true,
		TTY:       true,
	}

	return so, out
}

func (koe Kubik) newExecOptions(cmd []string, so exex.StreamOptions) exex.ExecOptions {
	r := exex.ExecOptions{Command: cmd, StreamOptions: so}

	r.Config = &koe.Config
	r.PodClient = koe.Clientset.CoreV1()
	r.Executor = &exex.DefaultRemoteExecutor{}

	return r
}



type Kubik struct {
	kubernetes.Clientset
	restclient.Config

	podCache map[string]coreV1.Pod
}

func PodToKey (p coreV1.Pod) string {
	return fmt.Sprintf("%s/%s", p.Namespace, p.Name)
}

func (k *Kubik) BuildCache(ui UIex) {
	pods, err := k.GetPods()
	ui.Logger.Error("GetPods: ", pods, err)
	if err != nil {
		panic("Failed to retrieve pods with error")
	}

	k.podCache = make(map[string]coreV1.Pod)
	for p := range pods {
		k.podCache[PodToKey(pods[p])] = pods[p]
	}
}

func (k *Kubik) GetPod(namespace, name string) coreV1.Pod {
	p, prs := k.podCache[fmt.Sprintf("%s/%s", namespace, name)]
	if prs == false {
		panic("Pod not found in cache")
	}
	return p
}

func NewKubik(ui UIex, kubeconfig string) Kubik {
	if kubeconfig == "" {
		kubeconfig = "/Users/myakovliev/kube_52b/admin/kubeconfig2.yaml"
	}

	k := Kubik{}
	k.Config = *getConfig(kubeconfig)
	k.Clientset = *getClientSet(&k.Config)

	ui.Logger.Error("Build cache")
	k.BuildCache(ui)
	ui.Logger.Error(k.podCache)
	ui.Logger.Error("Cache built")

	return k
}


func (k *Kubik) GetPods() ([]coreV1.Pod, error) {
	if len(k.podCache) != 0 {
		pods := []coreV1.Pod{}
		for _, v := range k.podCache {
			pods = append(pods, v)
		}
		return pods, nil
	}

	list, err := k.Clientset.CoreV1().Pods("openstack").List(metaV1.ListOptions{})
	if err != nil {
		return []coreV1.Pod{}, err
	}
	return list.Items, nil
}

func (k *Kubik) Run(pod coreV1.Pod, cmd []string) ([]byte, error) {
	if pod.Status.Phase != "Running" {
		return []byte{}, errors.Errorf("Pod %s/%s is not running.", pod.Namespace, pod.Name)
	}

	streamOpts, result := k.newStreamOptions(pod)
	execOpts := k.newExecOptions(cmd, streamOpts)
	err := execOpts.Run()
	if err != nil {
		return []byte{}, err
	}
	d, _ := ioutil.ReadAll(result)
	return d, nil
}
