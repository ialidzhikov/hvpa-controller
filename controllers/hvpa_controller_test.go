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
	"context"
	"fmt"
	"time"

	autoscalingv1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	autoscaling "k8s.io/api/autoscaling/v2beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	vpa_api "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
)

const timeout = time.Second * 5

var _ = Describe("#TestReconcile", func() {

	DescribeTable("##ReconcileHPAandVPA",
		func(instance *autoscalingv1alpha1.Hvpa) {

			replica := int32(1)
			deploytest := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deploy-test-1",
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &replica,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"name": "testDeployment",
						},
					},
					Template: v1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"name": "testDeployment",
							},
						},
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								v1.Container{
									Name:  "pause",
									Image: "k8s.gcr.io/pause-amd64:3.0",
								},
							},
						},
					},
				},
			}

			c := mgr.GetClient()
			// Create the test deployment
			err := c.Create(context.TODO(), deploytest)
			Expect(err).NotTo(HaveOccurred())

			// Create the Hvpa object and expect the Reconcile and HPA to be created
			err = c.Create(context.TODO(), instance)
			Expect(err).NotTo(HaveOccurred())
			defer c.Delete(context.TODO(), instance)

			hpaList := &autoscaling.HorizontalPodAutoscalerList{}
			hpa := &autoscaling.HorizontalPodAutoscaler{}
			Eventually(func() error {
				num := 0
				c.List(context.TODO(), hpaList)
				for _, obj := range hpaList.Items {
					if obj.GenerateName == "hvpa-1-" {
						num = num + 1
						hpa = obj.DeepCopy()
					}
				}
				if num == 1 {
					return nil
				}
				return fmt.Errorf("Error: Expected 1 HPA; found %v", len(hpaList.Items))
			}, timeout).Should(Succeed())

			vpaList := &vpa_api.VerticalPodAutoscalerList{}
			vpa := &vpa_api.VerticalPodAutoscaler{}
			Eventually(func() error {
				num := 0
				c.List(context.TODO(), vpaList)
				for _, obj := range vpaList.Items {
					if obj.GenerateName == "hvpa-1-" {
						num = num + 1
						vpa = obj.DeepCopy()
					}
				}
				if num == 1 {
					return nil
				}
				return fmt.Errorf("Error: Expected 1 VPA; found %v", len(vpaList.Items))
			}, timeout).Should(Succeed())

			// Delete the HPA and expect Reconcile to be called for HPA deletion
			Expect(c.Delete(context.TODO(), hpa)).NotTo(HaveOccurred())
			Eventually(func() error {
				num := 0
				c.List(context.TODO(), hpaList)
				for _, obj := range hpaList.Items {
					if obj.GenerateName == "hvpa-1-" {
						num = num + 1
						hpa = obj.DeepCopy()
					}
				}
				if num == 1 {
					return nil
				}
				return fmt.Errorf("Error: Expected 1 HPA; found %v", len(hpaList.Items))
			}, timeout).Should(Succeed())

			// Manually delete HPA & VPA since GC isn't enabled in the test control plane
			Eventually(func() error { return c.Delete(context.TODO(), hpa) }, timeout).
				Should(MatchError(fmt.Sprintf("horizontalpodautoscalers.autoscaling \"%s\" not found", hpa.Name)))
			Eventually(func() error { return c.Delete(context.TODO(), vpa) }, timeout).
				Should(MatchError(fmt.Sprintf("verticalpodautoscalers.autoscaling.k8s.io \"%s\" not found", vpa.Name)))

			// Delete the test deployment
			Expect(c.Delete(context.TODO(), deploytest)).NotTo(HaveOccurred())
		},
		Entry("hvpa", newHvpa("hvpa-1", "deploy-test-1", "label-1")),
	)
})
