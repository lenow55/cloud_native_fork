/*
This file is part of Cloud Native PostgreSQL.

Copyright (C) 2019-2021 EnterpriseDB Corporation.
*/

package controllers

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"

	v1 "github.com/EnterpriseDB/cloud-native-postgresql/api/v1"
	"github.com/EnterpriseDB/cloud-native-postgresql/pkg/certs"
)

var _ = Describe("cluster_status unit tests", func() {
	It("should make sure setCertExpiration works correctly", func() {
		var certExpirationDate string

		ctx := context.Background()
		namespace := newFakeNamespace()
		cluster := newFakeCNPCluster(namespace)
		secretName := rand.String(10)

		By("creating the required secret", func() {
			keyPair, err := certs.CreateRootCA("unittest.com", namespace)
			Expect(err).To(BeNil())
			secret := &corev1.Secret{
				Type: corev1.SecretTypeTLS,
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespace,
				},
				Data: map[string][]byte{
					corev1.TLSPrivateKeyKey: keyPair.Private,
					corev1.TLSCertKey:       keyPair.Certificate,
				},
			}

			err = k8sClient.Create(ctx, secret)
			Expect(err).To(BeNil())

			_, expDate, err := keyPair.IsExpiring()
			Expect(err).To(BeNil())

			certExpirationDate = expDate.String()
		})
		By("making sure that sets the status of the secret correctly", func() {
			cluster.Status.Certificates.Expirations = map[string]string{}
			err := clusterReconciler.setCertExpiration(ctx, cluster, secretName, namespace, corev1.TLSCertKey)
			Expect(err).To(BeNil())
			Expect(cluster.Status.Certificates.Expirations[secretName]).To(Equal(certExpirationDate))
		})
	})

	It("makes sure that getPgbouncerIntegrationStatus returns the correct secret name without duplicates", func() {
		ctx := context.Background()
		namespace := newFakeNamespace()
		cluster := newFakeCNPCluster(namespace)
		pooler1 := *newFakePooler(cluster)
		pooler2 := *newFakePooler(cluster)
		Expect(pooler1.Name).ToNot(Equal(pooler2.Name))
		poolerList := v1.PoolerList{Items: []v1.Pooler{pooler1, pooler2}}

		intStatus, err := clusterReconciler.getPgbouncerIntegrationStatus(ctx, cluster, poolerList)
		Expect(err).To(BeNil())
		Expect(intStatus.Secrets).To(HaveLen(1))
	})

	It("makes sure getObjectResourceVersion returns the correct object version", func() {
		ctx := context.Background()
		namespace := newFakeNamespace()
		cluster := newFakeCNPCluster(namespace)
		pooler := newFakePooler(cluster)

		version, err := clusterReconciler.getObjectResourceVersion(ctx, cluster, pooler.Name, &v1.Pooler{})
		Expect(err).To(BeNil())
		Expect(version).To(Equal(pooler.ResourceVersion))
	})

	It("makes sure setPrimaryInstance works correctly", func() {
		const podName = "test-pod"
		ctx := context.Background()
		namespace := newFakeNamespace()
		cluster := newFakeCNPCluster(namespace)
		Expect(cluster.Status.TargetPrimaryTimestamp).To(BeEmpty())

		By("setting the primaryInstance and making sure the passed object is updated", func() {
			err := clusterReconciler.setPrimaryInstance(ctx, cluster, podName)
			Expect(err).To(BeNil())
			Expect(cluster.Status.TargetPrimaryTimestamp).ToNot(BeEmpty())
			Expect(cluster.Status.TargetPrimary).To(Equal(podName))
		})

		By("making sure the remote resource is updated", func() {
			remoteCluster := &v1.Cluster{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, remoteCluster)
			Expect(err).To(BeNil())
			Expect(remoteCluster.Status.TargetPrimaryTimestamp).ToNot(BeEmpty())
			Expect(remoteCluster.Status.TargetPrimary).To(Equal(podName))
		})
	})

	It("makes sure RegisterPhase works correctly", func() {
		const phaseReason = "testing"
		ctx := context.Background()
		namespace := newFakeNamespace()
		cluster := newFakeCNPCluster(namespace)

		By("registering the phase and making sure the passed object is updated", func() {
			err := clusterReconciler.RegisterPhase(ctx, cluster, v1.PhaseSwitchover, phaseReason)
			Expect(err).To(BeNil())
			Expect(cluster.Status.Phase).To(Equal(v1.PhaseSwitchover))
			Expect(cluster.Status.PhaseReason).To(Equal(phaseReason))
		})

		By("making sure the remote resource is updated", func() {
			remoteCluster := &v1.Cluster{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, remoteCluster)
			Expect(err).To(BeNil())
			Expect(remoteCluster.Status.Phase).To(Equal(v1.PhaseSwitchover))
			Expect(remoteCluster.Status.PhaseReason).To(Equal(phaseReason))
		})
	})
})
