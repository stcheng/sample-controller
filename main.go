/*
Copyright 2017 The Kubernetes Authors.

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

package main

import (
	"flag"
	"time"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	clientset "k8s.io/sample-controller/pkg/generated/clientset/versioned"
	informers "k8s.io/sample-controller/pkg/generated/informers/externalversions"
	"k8s.io/sample-controller/pkg/signals"
)

var (
	masterURL   string
	kubeconfig  string
	masterStage bool
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	// get in-cluster config
	inCfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		klog.Fatalf("Error building in-cluster kubeconfig: %s", err.Error())
	}

	inKubeClient, err := kubernetes.NewForConfig(inCfg)
	if err != nil {
		klog.Fatalf("Error building in-cluster kubernetes clientset: %s", err.Error())
	}

	inExampleClient, err := clientset.NewForConfig(inCfg)
	if err != nil {
		klog.Fatalf("Error building in-cluster example clientset: %s", err.Error())
	}

	// init in-cluster
	inKubeInformerFactory := kubeinformers.NewSharedInformerFactory(inKubeClient, time.Second*30)
	inExampleInformerFactory := informers.NewSharedInformerFactory(inExampleClient, time.Second*30)

	if masterStage {
		klog.Infof("Running the master controller ")

		// get out-cluster config
		outCfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
		if err != nil {
			klog.Fatalf("Error building kubeconfig: %s", err.Error())
		}

		outKubeClient, err := kubernetes.NewForConfig(outCfg)
		if err != nil {
			klog.Fatalf("Error building kubernetes clientset: %s", err.Error())
		}

		outExampleClient, err := clientset.NewForConfig(outCfg)
		if err != nil {
			klog.Fatalf("Error building example clientset: %s", err.Error())
		}

		// init out-cluster
		outKubeInformerFactory := kubeinformers.NewSharedInformerFactory(outKubeClient, time.Second*30)
		outExampleInformerFactory := informers.NewSharedInformerFactory(outExampleClient, time.Second*30)

		// init controller
		controller := NewController(outKubeClient, outExampleClient,
			inKubeClient, inExampleClient,

			outKubeInformerFactory.Apps().V1().Deployments(),
			inExampleInformerFactory.Samplecontroller().V1alpha1().Foos(),
			outExampleInformerFactory.Samplecontroller().V1alpha1().Bars())

		inKubeInformerFactory.Start(stopCh)
		inExampleInformerFactory.Start(stopCh)

		// notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(stopCh)
		// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
		outKubeInformerFactory.Start(stopCh)
		outExampleInformerFactory.Start(stopCh)

		if err = controller.Run(4, stopCh); err != nil {
			klog.Fatalf("Error running controller: %s", err.Error())
		}
	} else {
		klog.Infof("Running the sub controller")

		// init controller
		controller := NewBarController(
			inKubeClient, inExampleClient,

			inKubeInformerFactory.Apps().V1().Deployments(),
			inExampleInformerFactory.Samplecontroller().V1alpha1().Bars())

		inKubeInformerFactory.Start(stopCh)
		inExampleInformerFactory.Start(stopCh)

		if err = controller.Run(2, stopCh); err != nil {
			klog.Fatalf("Error running controller: %s", err.Error())
		}
	}

}

func init() {
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	flag.BoolVar(&masterStage, "master-stage", false, "indicate if the controller is used at master stage")
}
