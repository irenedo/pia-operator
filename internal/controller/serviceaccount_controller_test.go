package controller_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/irenedo/pia-operator/internal/controller"
	awsclientmocks "github.com/irenedo/pia-operator/pkg/awsclient/mocks"
	k8sclientmocks "github.com/irenedo/pia-operator/pkg/k8sclient/mocks"
)

var _ = Describe("ServiceAccountReconciler", func() {
	var (
		ctx           context.Context
		mockAWSClient *awsclientmocks.MockAWSClient
		mockK8sClient *k8sclientmocks.MockCli
		reconciler    *controller.ServiceAccountReconciler
		fakeClient    client.Client
		scheme        *runtime.Scheme
	)

	BeforeEach(func() {
		ctx = context.Background()

		scheme = runtime.NewScheme()
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()

		mockAWSClient = awsclientmocks.NewMockAWSClient(GinkgoT())
		mockK8sClient = k8sclientmocks.NewMockCli(GinkgoT())

		reconciler = &controller.ServiceAccountReconciler{
			Client:      fakeClient,
			Log:         log.Log,
			Scheme:      scheme,
			AWSRegion:   "us-west-2",
			ClusterName: "test-cluster",
			AWSClient:   mockAWSClient,
			K8sClient:   mockK8sClient,
		}
	})

	Describe("Reconcile", func() {
		Context("when ServiceAccount does not exist", func() {
			It("should return no error and empty result", func() {
				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      "nonexistent",
						Namespace: "default",
					},
				}

				notFoundError := k8errors.NewNotFound(corev1.Resource("serviceaccounts"), "nonexistent")
				mockK8sClient.On("GetServiceAccount", ctx, "default", "nonexistent").Return(nil, notFoundError)

				result, err := reconciler.Reconcile(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				mockK8sClient.AssertExpectations(GinkgoT())
			})
		})

		Context("when ServiceAccount exists but has no relevant annotations", func() {
			It("should return no error and empty result", func() {
				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "test-sa",
						Namespace:   "default",
						Annotations: nil,
					},
				}

				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      sa.Name,
						Namespace: sa.Namespace,
					},
				}

				mockK8sClient.On("GetServiceAccount", ctx, sa.Namespace, sa.Name).Return(sa, nil)

				result, err := reconciler.Reconcile(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				mockK8sClient.AssertExpectations(GinkgoT())
			})
		})

		Context("when ServiceAccount has role annotation", func() {
			It("should handle create association flow", func() {
				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sa",
						Namespace: "default",
						Annotations: map[string]string{
							controller.PodIdentityAssociationRoleAnnotation: "arn:aws:iam::123456789012:role/test-role",
						},
					},
				}

				// Create the ServiceAccount in the fake client for the finalizer update
				Expect(fakeClient.Create(ctx, sa)).To(Succeed())

				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      sa.Name,
						Namespace: sa.Namespace,
					},
				}

				mockK8sClient.On("GetServiceAccount", ctx, sa.Namespace, sa.Name).Return(sa, nil)
				mockAWSClient.On("AssociationExists", ctx, sa).Return(false, nil)
				mockAWSClient.On("CreatePodIdentityAssociation", ctx, sa, "arn:aws:iam::123456789012:role/test-role", "", true).Return("assoc-123", nil)

				// Mock the K8sClient update for the association ID
				mockK8sClient.On("UpdateServiceAccount", ctx, sa).Return(nil)

				result, err := reconciler.Reconcile(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				mockK8sClient.AssertExpectations(GinkgoT())
				mockAWSClient.AssertExpectations(GinkgoT())
			})

			It("should handle update association flow", func() {
				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sa",
						Namespace: "default",
						Annotations: map[string]string{
							controller.PodIdentityAssociationRoleAnnotation: "arn:aws:iam::123456789012:role/updated-role",
						},
					},
				}

				// Create the ServiceAccount in the fake client
				Expect(fakeClient.Create(ctx, sa)).To(Succeed())

				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      sa.Name,
						Namespace: sa.Namespace,
					},
				}

				mockK8sClient.On("GetServiceAccount", ctx, sa.Namespace, sa.Name).Return(sa, nil)
				mockAWSClient.On("AssociationExists", ctx, sa).Return(true, nil)
				mockAWSClient.On("UpdatePodIdentityAssociation", ctx, sa, "arn:aws:iam::123456789012:role/updated-role", "", true).Return("assoc-456", nil)

				// Mock the K8sClient update for the association ID
				mockK8sClient.On("UpdateServiceAccount", ctx, sa).Return(nil)

				result, err := reconciler.Reconcile(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				mockK8sClient.AssertExpectations(GinkgoT())
				mockAWSClient.AssertExpectations(GinkgoT())
			})

			It("should handle AWS errors", func() {
				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sa",
						Namespace: "default",
						Annotations: map[string]string{
							controller.PodIdentityAssociationRoleAnnotation: "arn:aws:iam::123456789012:role/test-role",
						},
					},
				}

				// Create the ServiceAccount in the fake client
				Expect(fakeClient.Create(ctx, sa)).To(Succeed())

				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      sa.Name,
						Namespace: sa.Namespace,
					},
				}

				expectedError := errors.New("AWS error")

				mockK8sClient.On("GetServiceAccount", ctx, sa.Namespace, sa.Name).Return(sa, nil)
				mockAWSClient.On("AssociationExists", ctx, sa).Return(false, expectedError)

				result, err := reconciler.Reconcile(ctx, req)

				// The reconciler should return the result from error handler, which will handle the error
				// and return a requeue result with no error to the controller-runtime
				Expect(err).ToNot(HaveOccurred())                     // Error handler handles the error internally
				Expect(result.RequeueAfter).To(BeNumerically(">", 0)) // Should requeue after some time

				mockK8sClient.AssertExpectations(GinkgoT())
				mockAWSClient.AssertExpectations(GinkgoT())
			})

			It("should handle tagging enabled by default (no annotation)", func() {
				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sa",
						Namespace: "default",
						Annotations: map[string]string{
							controller.PodIdentityAssociationRoleAnnotation: "arn:aws:iam::123456789012:role/test-role",
						},
					},
				}

				// Create the ServiceAccount in the fake client
				Expect(fakeClient.Create(ctx, sa)).To(Succeed())

				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      sa.Name,
						Namespace: sa.Namespace,
					},
				}

				mockK8sClient.On("GetServiceAccount", ctx, sa.Namespace, sa.Name).Return(sa, nil)
				mockAWSClient.On("AssociationExists", ctx, sa).Return(false, nil)
				// Expect tagging to be enabled (true) by default
				mockAWSClient.On("CreatePodIdentityAssociation", ctx, sa, "arn:aws:iam::123456789012:role/test-role", "", true).Return("assoc-123", nil)
				mockK8sClient.On("UpdateServiceAccount", ctx, sa).Return(nil)

				result, err := reconciler.Reconcile(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				mockK8sClient.AssertExpectations(GinkgoT())
				mockAWSClient.AssertExpectations(GinkgoT())
			})

			It("should handle tagging explicitly enabled (annotation set to true)", func() {
				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sa",
						Namespace: "default",
						Annotations: map[string]string{
							controller.PodIdentityAssociationRoleAnnotation:    "arn:aws:iam::123456789012:role/test-role",
							controller.PodIdentityAssociationTaggingAnnotation: "true",
						},
					},
				}

				// Create the ServiceAccount in the fake client
				Expect(fakeClient.Create(ctx, sa)).To(Succeed())

				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      sa.Name,
						Namespace: sa.Namespace,
					},
				}

				mockK8sClient.On("GetServiceAccount", ctx, sa.Namespace, sa.Name).Return(sa, nil)
				mockAWSClient.On("AssociationExists", ctx, sa).Return(false, nil)
				// Expect tagging to be enabled (true)
				mockAWSClient.On("CreatePodIdentityAssociation", ctx, sa, "arn:aws:iam::123456789012:role/test-role", "", true).Return("assoc-123", nil)
				mockK8sClient.On("UpdateServiceAccount", ctx, sa).Return(nil)

				result, err := reconciler.Reconcile(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				mockK8sClient.AssertExpectations(GinkgoT())
				mockAWSClient.AssertExpectations(GinkgoT())
			})

			It("should handle tagging disabled (annotation set to false)", func() {
				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sa",
						Namespace: "default",
						Annotations: map[string]string{
							controller.PodIdentityAssociationRoleAnnotation:    "arn:aws:iam::123456789012:role/test-role",
							controller.PodIdentityAssociationTaggingAnnotation: "false",
						},
					},
				}

				// Create the ServiceAccount in the fake client
				Expect(fakeClient.Create(ctx, sa)).To(Succeed())

				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      sa.Name,
						Namespace: sa.Namespace,
					},
				}

				mockK8sClient.On("GetServiceAccount", ctx, sa.Namespace, sa.Name).Return(sa, nil)
				mockAWSClient.On("AssociationExists", ctx, sa).Return(false, nil)
				// Expect tagging to be disabled (false)
				mockAWSClient.On("CreatePodIdentityAssociation", ctx, sa, "arn:aws:iam::123456789012:role/test-role", "", false).Return("assoc-123", nil)
				mockK8sClient.On("UpdateServiceAccount", ctx, sa).Return(nil)

				result, err := reconciler.Reconcile(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				mockK8sClient.AssertExpectations(GinkgoT())
				mockAWSClient.AssertExpectations(GinkgoT())
			})

			It("should handle tagging enabled with non-false value", func() {
				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sa",
						Namespace: "default",
						Annotations: map[string]string{
							controller.PodIdentityAssociationRoleAnnotation:    "arn:aws:iam::123456789012:role/test-role",
							controller.PodIdentityAssociationTaggingAnnotation: "anything-not-false",
						},
					},
				}

				// Create the ServiceAccount in the fake client
				Expect(fakeClient.Create(ctx, sa)).To(Succeed())

				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      sa.Name,
						Namespace: sa.Namespace,
					},
				}

				mockK8sClient.On("GetServiceAccount", ctx, sa.Namespace, sa.Name).Return(sa, nil)
				mockAWSClient.On("AssociationExists", ctx, sa).Return(false, nil)
				// Expect tagging to be enabled (true) for any value that's not "false"
				mockAWSClient.On("CreatePodIdentityAssociation", ctx, sa, "arn:aws:iam::123456789012:role/test-role", "", true).Return("assoc-123", nil)
				mockK8sClient.On("UpdateServiceAccount", ctx, sa).Return(nil)

				result, err := reconciler.Reconcile(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				mockK8sClient.AssertExpectations(GinkgoT())
				mockAWSClient.AssertExpectations(GinkgoT())
			})

			It("should handle update with tagging disabled", func() {
				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sa",
						Namespace: "default",
						Annotations: map[string]string{
							controller.PodIdentityAssociationRoleAnnotation:    "arn:aws:iam::123456789012:role/updated-role",
							controller.PodIdentityAssociationTaggingAnnotation: "false",
						},
					},
				}

				// Create the ServiceAccount in the fake client
				Expect(fakeClient.Create(ctx, sa)).To(Succeed())

				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      sa.Name,
						Namespace: sa.Namespace,
					},
				}

				mockK8sClient.On("GetServiceAccount", ctx, sa.Namespace, sa.Name).Return(sa, nil)
				mockAWSClient.On("AssociationExists", ctx, sa).Return(true, nil)
				// Expect tagging to be disabled (false) in the update call
				mockAWSClient.On("UpdatePodIdentityAssociation", ctx, sa, "arn:aws:iam::123456789012:role/updated-role", "", false).Return("assoc-456", nil)
				mockK8sClient.On("UpdateServiceAccount", ctx, sa).Return(nil)

				result, err := reconciler.Reconcile(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				mockK8sClient.AssertExpectations(GinkgoT())
				mockAWSClient.AssertExpectations(GinkgoT())
			})
		})

		Context("when ServiceAccount is being deleted", func() {
			It("should handle deletion with finalizer", func() {
				deletionTime := metav1.Now()
				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sa",
						Namespace: "default",
						Annotations: map[string]string{
							controller.PodIdentityAssociationRoleAnnotation: "arn:aws:iam::123456789012:role/test-role",
						},
						Finalizers:        []string{controller.PodIdentityAssociationFinalizer},
						DeletionTimestamp: &deletionTime,
					},
				}

				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      sa.Name,
						Namespace: sa.Namespace,
					},
				}

				mockK8sClient.On("GetServiceAccount", ctx, sa.Namespace, sa.Name).Return(sa, nil)
				mockAWSClient.On("DeletePodIdentityAssociation", ctx, sa).Return(nil)
				mockK8sClient.On("UpdateServiceAccount", ctx, sa).Return(nil)

				result, err := reconciler.Reconcile(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				mockK8sClient.AssertExpectations(GinkgoT())
				mockAWSClient.AssertExpectations(GinkgoT())
			})

			It("should handle deletion without finalizer", func() {
				deletionTime := metav1.Now()
				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "test-sa",
						Namespace:         "default",
						DeletionTimestamp: &deletionTime,
					},
				}

				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      sa.Name,
						Namespace: sa.Namespace,
					},
				}

				mockK8sClient.On("GetServiceAccount", ctx, sa.Namespace, sa.Name).Return(sa, nil)

				result, err := reconciler.Reconcile(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				mockK8sClient.AssertExpectations(GinkgoT())
			})

			It("should remove tagging annotation during deletion cleanup", func() {
				deletionTime := metav1.Now()
				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sa",
						Namespace: "default",
						Annotations: map[string]string{
							controller.PodIdentityAssociationRoleAnnotation:       "arn:aws:iam::123456789012:role/test-role",
							controller.PodIdentityAssociationAssumeRoleAnnotation: "arn:aws:iam::123456789012:role/assume-role",
							controller.PodIdentityAssociationTaggingAnnotation:    "false",
							controller.PodIdentityAssociationIDAnnotation:         "assoc-123",
						},
						Finalizers:        []string{controller.PodIdentityAssociationFinalizer},
						DeletionTimestamp: &deletionTime,
					},
				}

				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      sa.Name,
						Namespace: sa.Namespace,
					},
				}

				mockK8sClient.On("GetServiceAccount", ctx, sa.Namespace, sa.Name).Return(sa, nil)
				mockAWSClient.On("DeletePodIdentityAssociation", ctx, sa).Return(nil)

				// Mock the K8sClient update call and verify all annotations are removed
				mockK8sClient.On("UpdateServiceAccount", ctx, sa).Run(func(args mock.Arguments) {
					updatedSA, ok := args.Get(1).(*corev1.ServiceAccount)
					Expect(ok).To(BeTrue())
					// Verify that all PIA-related annotations are removed
					Expect(updatedSA.Annotations).ToNot(HaveKey(controller.PodIdentityAssociationAssumeRoleAnnotation))
					Expect(updatedSA.Annotations).ToNot(HaveKey(controller.PodIdentityAssociationIDAnnotation))
					Expect(updatedSA.Annotations).ToNot(HaveKey(controller.PodIdentityAssociationTaggingAnnotation))
				}).Return(nil)

				result, err := reconciler.Reconcile(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				mockK8sClient.AssertExpectations(GinkgoT())
				mockAWSClient.AssertExpectations(GinkgoT())
			})
		})

		Context("when ServiceAccount needs cleanup", func() {
			It("should cleanup when finalizer exists but no role annotation", func() {
				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "test-sa",
						Namespace:  "default",
						Finalizers: []string{controller.PodIdentityAssociationFinalizer},
					},
				}

				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      sa.Name,
						Namespace: sa.Namespace,
					},
				}

				mockK8sClient.On("GetServiceAccount", ctx, sa.Namespace, sa.Name).Return(sa, nil)
				mockAWSClient.On("DeletePodIdentityAssociation", ctx, sa).Return(nil)
				mockK8sClient.On("UpdateServiceAccount", ctx, sa).Return(nil)

				result, err := reconciler.Reconcile(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				mockK8sClient.AssertExpectations(GinkgoT())
				mockAWSClient.AssertExpectations(GinkgoT())
			})

			It("should remove tagging annotation during cleanup when no role annotation", func() {
				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-sa",
						Namespace: "default",
						Annotations: map[string]string{
							controller.PodIdentityAssociationTaggingAnnotation: "false",
							controller.PodIdentityAssociationIDAnnotation:      "assoc-456",
							"other-annotation": "should-remain",
						},
						Finalizers: []string{controller.PodIdentityAssociationFinalizer},
					},
				}

				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      sa.Name,
						Namespace: sa.Namespace,
					},
				}

				mockK8sClient.On("GetServiceAccount", ctx, sa.Namespace, sa.Name).Return(sa, nil)
				mockAWSClient.On("DeletePodIdentityAssociation", ctx, sa).Return(nil)

				// Mock the K8sClient update call and verify PIA annotations are removed but others remain
				mockK8sClient.On("UpdateServiceAccount", ctx, sa).Run(func(args mock.Arguments) {
					updatedSA, ok := args.Get(1).(*corev1.ServiceAccount)
					Expect(ok).To(BeTrue())
					// Verify that PIA-related annotations are removed
					Expect(updatedSA.Annotations).ToNot(HaveKey(controller.PodIdentityAssociationTaggingAnnotation))
					Expect(updatedSA.Annotations).ToNot(HaveKey(controller.PodIdentityAssociationIDAnnotation))
					// Verify that non-PIA annotations remain
					Expect(updatedSA.Annotations).To(HaveKeyWithValue("other-annotation", "should-remain"))
				}).Return(nil)

				result, err := reconciler.Reconcile(ctx, req)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(ctrl.Result{}))

				mockK8sClient.AssertExpectations(GinkgoT())
				mockAWSClient.AssertExpectations(GinkgoT())
			})
		})

		Context("when getting ServiceAccount fails", func() {
			It("should return error for non-NotFound errors", func() {
				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      "test-sa",
						Namespace: "default",
					},
				}

				expectedError := errors.New("internal server error")

				mockK8sClient.On("GetServiceAccount", ctx, "default", "test-sa").Return(nil, expectedError)

				result, err := reconciler.Reconcile(ctx, req)

				Expect(err).To(Equal(expectedError))
				Expect(result).To(Equal(ctrl.Result{}))

				mockK8sClient.AssertExpectations(GinkgoT())
			})
		})
	})
})
