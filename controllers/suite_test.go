/*
Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved.

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

package controllers

import (
	"path/filepath"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	autoscalingv1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	autoscaling "k8s.io/api/autoscaling/v2beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var mgr manager.Manager
var stopMgr chan struct{}
var mgrStopped *sync.WaitGroup

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{envtest.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = autoscalingv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).ToNot(HaveOccurred())
	Expect(k8sClient).ToNot(BeNil())

	// Setup the Manager.
	mgr, err = ctrl.NewManager(cfg, ctrl.Options{})
	Expect(err).NotTo(HaveOccurred())

	reconciler := HvpaReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}

	err = reconciler.SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	stopMgr, mgrStopped = StartTestManager(mgr, &GomegaWithT{})

	close(done)
}, 60)

var _ = AfterSuite(func() {
	close(stopMgr)
	mgrStopped.Wait()

	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

// SetupTestReconcile returns a reconcile.Reconcile implementation that delegates to inner and
// writes the request to requests after Reconcile is finished.
func SetupTestReconcile(inner reconcile.Reconciler) (reconcile.Reconciler, chan reconcile.Request) {
	requests := make(chan reconcile.Request)
	fn := reconcile.Func(func(req reconcile.Request) (reconcile.Result, error) {
		result, err := inner.Reconcile(req)
		requests <- req
		return result, err
	})
	return fn, requests
}

// StartTestManager adds recFn
func StartTestManager(mgr manager.Manager, g *GomegaWithT) (chan struct{}, *sync.WaitGroup) {
	stop := make(chan struct{})
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		Expect(mgr.Start(stop)).NotTo(HaveOccurred())
	}()
	return stop, wg
}

func newHvpa(name, target, labelVal string) *autoscalingv1alpha1.Hvpa {
	replica := int32(1)
	util := int32(70)
	updateMode := autoscalingv1alpha1.UpdateModeAuto

	instance := &autoscalingv1alpha1.Hvpa{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: autoscalingv1alpha1.HvpaSpec{
			TargetRef: &autoscaling.CrossVersionObjectReference{
				Kind:       "Deployment",
				Name:       target,
				APIVersion: "apps/v1",
			},
			Hpa: autoscalingv1alpha1.HpaSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"hpaKey": labelVal,
					},
				},
				UpdatePolicy: &autoscalingv1alpha1.UpdatePolicy{
					UpdateMode: &updateMode,
				},
				Template: autoscalingv1alpha1.HpaTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"hpaKey": labelVal,
						},
					},
					Spec: autoscalingv1alpha1.HpaTemplateSpec{
						MinReplicas: &replica,
						MaxReplicas: 2,
						Metrics: []autoscaling.MetricSpec{
							autoscaling.MetricSpec{
								Type: autoscaling.ResourceMetricSourceType,
								Resource: &autoscaling.ResourceMetricSource{
									Name:                     v1.ResourceCPU,
									TargetAverageUtilization: &util,
								},
							},
						},
					},
				},
			},
			Vpa: autoscalingv1alpha1.VpaSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"vpaKey": labelVal,
					},
				},
				UpdatePolicy: &autoscalingv1alpha1.UpdatePolicy{
					UpdateMode: &updateMode,
				},
				Template: autoscalingv1alpha1.VpaTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"vpaKey": labelVal,
						},
					},
				},
			},
		},
	}

	return instance
}
