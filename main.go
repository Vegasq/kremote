package main

import (
	"bytes"
	"fmt"
	"github.com/sirupsen/logrus"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	exex "k8s.io/kubectl/pkg/cmd/exec"
)


func GetConfig() (*restclient.Config) {
	config, confErr := clientcmd.BuildConfigFromFlags(
		"", "/Users/myakovliev/kube_52b/admin/kubeconfig2.yaml")

	if confErr != nil {
		logrus.Fatalln("Can't read config: %s.\n", confErr)
	}

	config.APIPath = "/api"

	var Unversioned = schema.GroupVersion{Group: "", Version: "v1"}
	config.GroupVersion = &Unversioned
	config.NegotiatedSerializer = scheme.Codecs

	return config
}

func GetClientSet(config *restclient.Config) *kubernetes.Clientset {
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


type kubernetesObjectEmitter struct{
	kubernetes.Clientset
	restclient.Config
}

func NewKubernetesObjectEmitter(cs kubernetes.Clientset, c restclient.Config) kubernetesObjectEmitter {
	koe := kubernetesObjectEmitter{}
	koe.Clientset = cs
	koe.Config = c

	return koe
}

func (kubernetesObjectEmitter) newStreamOptions(pod coreV1.Pod) (exex.StreamOptions, *bytes.Buffer) {
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
	}

	return so, out
}

func (koe kubernetesObjectEmitter) newExecOptions(cmd []string, so exex.StreamOptions) exex.ExecOptions {
	r := exex.ExecOptions{Command: cmd, StreamOptions: so}

	r.Config = &koe.Config
	r.PodClient = koe.Clientset.CoreV1()
	r.Executor = &exex.DefaultRemoteExecutor{}

	return r
}

type runnerTask struct {
	Pod coreV1.Pod
	Output string
	Error string
}

func runner(context kubernetesObjectEmitter, pod coreV1.Pod, out chan runnerTask){
	if pod.Status.Phase != "Running" {
		out <- runnerTask{Pod: pod, Error: "Pod is not running"}
		return
	}

	streamOpts, result := context.newStreamOptions(pod)
	execOpts := context.newExecOptions([]string{"uname", "-a"}, streamOpts)
	err := execOpts.Run()
	if err != nil {
		out <- runnerTask{Pod: pod, Error: err.Error()}
	} else {
		out <- runnerTask{Pod: pod, Output: result.String()}
	}

}

func main(){
	cfg := GetConfig()
	clientSet := GetClientSet(cfg)

	list, err := clientSet.CoreV1().Pods("").List(metaV1.ListOptions{})

	if err != nil {
		logrus.Fatalln("PodList Error", err)
	}

	koe := NewKubernetesObjectEmitter(*clientSet, *cfg)
	output := make(chan runnerTask)

	for i := range list.Items {
		go runner(koe, list.Items[i], output)
	}
	for range list.Items {
		o := <- output
		fmt.Printf("Name: %s\nOut: %s\nErr: %s\n---\n", o.Pod.Name, o.Output, o.Error)
	}
}
