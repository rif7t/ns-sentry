/*
Copyright 2026.

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

package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// NamespaceReconciler reconciles a Namespace object
type NamespaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=namespaces/finalizers,verbs=update

// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=resourcequotas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Namespace object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.1/pkg/reconcile
func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	var ns corev1.Namespace
	if err := r.Get(ctx, req.NamespacedName, &ns); err != nil {
		// If the namespace was deleted, we don't need to do anything
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	systemNamespaces := map[string]bool{
		"kube-system":     true,
		"kube-public":     true,
		"kube-node-lease": true,
		"talos-system":    true,
		"metallb-system":  true,
		"nginx-ingress":   true,
	}

	if systemNamespaces[ns.Name] {
		l.Info("Skipping system namespace", "namespace", ns.Name)
		return ctrl.Result{}, nil
	}

	if !ns.DeletionTimestamp.IsZero() {
		l.Info("Namespace is being deleted, skipping hardening", "namespace", ns.Name)
		return ctrl.Result{}, nil
	}

	l.Info("Reconciling and hardening namespace", "namespace", ns.Name)
	if err := r.hardenNamespace(ctx, &ns); err != nil {
		l.Error(err, "Failed to harden namespace", "namespace", ns.Name)
		return ctrl.Result{}, err
	}

	l.Info("Successfully ensured policies in namespace", "namespace", ns.Name)
	return ctrl.Result{}, nil
}

func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Owns(&corev1.ResourceQuota{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Complete(r)
}
func (r *NamespaceReconciler) hardenNamespace(ctx context.Context, ns *corev1.Namespace) error {
	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: "enterprise-quota", Namespace: ns.Name},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, quota, func() error {
		quota.Spec = corev1.ResourceQuotaSpec{
			Hard: corev1.ResourceList{
				corev1.ResourceRequestsCPU:    resource.MustParse("500m"),
				corev1.ResourceRequestsMemory: resource.MustParse("512Mi"),
				corev1.ResourceLimitsCPU:      resource.MustParse("1"),
				corev1.ResourceLimitsMemory:   resource.MustParse("1Gi"),
			},
		}
		return controllerutil.SetControllerReference(ns, quota, r.Scheme)
	})
	if err != nil {
		return err
	}
	udpProtocol := corev1.ProtocolUDP
	dnsPort := intstr.FromInt(53)
	netPol := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "default-deny-all", Namespace: ns.Name},
	}
	_, err = controllerutil.CreateOrUpdate(ctx, r.Client, netPol, func() error {
		netPol.Spec = networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "kube-system",
								},
							},
							// Specifically to the DNS pods
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"k8s-app": "kube-dns",
								},
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &udpProtocol,
							Port:     &dnsPort,
						},
					},
				},
			},
		}
		return controllerutil.SetControllerReference(ns, netPol, r.Scheme)
	})
	return err
}
