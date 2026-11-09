/*
Copyright 2025.

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
	"fmt"
	"maps"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	ravendbv1 "ravendb-operator/api/v1"
	"ravendb-operator/pkg/clientcert"
	"ravendb-operator/pkg/common"
	"ravendb-operator/pkg/ravendb"
)

const (
	clientPFXKey = "client.pfx"
	caCertKey    = "ca.crt"
	// previousThumbprintAnnotation names the certificate the one in the Secret renewed. It stays registered,
	// so applications that have not reloaded the Secret yet keep working until it expires.
	previousThumbprintAnnotation = "ravendb.ravendb.io/previous-thumbprint"
)

/*
RavenDBClientCertificateReconciler keeps one application certificate registered in RavenDB and stored in a Secret.

The Secret is the source of truth: its client.pfx is the current certificate. Every reconcile
 1. issues a new certificate when the Secret has none (it was never written, or it was lost) or the current one
    is due for renewal,
 2. makes the current certificate, and the one it renewed, the only registrations under the resource's name,
    with the clearance and permissions from the spec; every other one is revoked,
 3. writes the Secret.

Registering before writing the Secret means applications never read a certificate RavenDB does not trust, and a
certificate registered by an attempt that failed before step 3 is revoked by the next one.
*/
type RavenDBClientCertificateReconciler struct {
	client.Client
	// APIReader reads the Secret past the cache, so a reconcile never acts on an older certificate than the one
	// the previous reconcile wrote.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	Recorder  record.EventRecorder
	Connect   connectFunc
	Now       func() time.Time
}

// +kubebuilder:rbac:groups=ravendb.ravendb.io,resources=ravendbclientcertificates,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=ravendb.ravendb.io,resources=ravendbclientcertificates/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ravendb.ravendb.io,resources=ravendbclientcertificates/finalizers,verbs=update
func (r *RavenDBClientCertificateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cc ravendbv1.RavenDBClientCertificate
	if err := r.Get(ctx, req.NamespacedName, &cc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !cc.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.finalize(ctx, &cc)
	}
	if err := addFinalizer(ctx, r.Client, &cc); err != nil {
		return ctrl.Result{}, err
	}

	original := cc.DeepCopy()
	renewAt, err := r.ensureCertificate(ctx, &cc)
	if err != nil {
		log.FromContext(ctx).Info("Client certificate not ready", "cause", err.Error())
	}
	cc.Status.ObservedGeneration = cc.Generation
	requeueAfter := recordOutcome(&cc.Status.Conditions, cc.Generation, err,
		fmt.Sprintf("Certificate %s is registered and stored in Secret %q", cc.Status.Thumbprint, cc.Spec.SecretName))
	if err == nil {
		requeueAfter = min(requeueAfter, max(renewAt.Sub(r.Now()), retryInterval))
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, patchStatus(ctx, r.Client, original, &cc)
}

// ensureCertificate brings RavenDB and the Secret in line with the spec and returns when to renew.
func (r *RavenDBClientCertificateReconciler) ensureCertificate(
	ctx context.Context, cc *ravendbv1.RavenDBClientCertificate,
) (time.Time, error) {
	cluster, rc, err := connect(ctx, r.Client, r.Connect, cc.Namespace, cc.Spec.ClusterRef)
	if err != nil {
		return time.Time{}, err
	}
	secret, err := r.ownedSecret(ctx, cc)
	if err != nil {
		return time.Time{}, err
	}
	caPEM, err := ravendb.CACertPEM(ctx, r.Client, cluster)
	if err != nil {
		return time.Time{}, err
	}

	validity := cc.Spec.Validity.Duration
	current, previous := storedCertificate(secret)
	if current == nil || !r.Now().Before(current.RenewAt(validity)) {
		if current, previous, err = r.issue(cc, current); err != nil {
			return time.Time{}, err
		}
	}

	if err := r.syncRegistrations(ctx, rc, cc, current, previous); err != nil {
		return time.Time{}, err
	}
	if err := r.writeSecret(ctx, cc, secret, current.PFX, caPEM, previous); err != nil {
		return time.Time{}, err
	}

	cc.Status.Thumbprint = current.Thumbprint()
	cc.Status.NotAfter = &metav1.Time{Time: current.X509.NotAfter}
	return current.RenewAt(validity), nil
}

// ownedSecret returns the Secret named in the spec, nil when it does not exist yet. A Secret this resource
// does not own is never overwritten.
func (r *RavenDBClientCertificateReconciler) ownedSecret(
	ctx context.Context, cc *ravendbv1.RavenDBClientCertificate,
) (*corev1.Secret, error) {
	var secret corev1.Secret
	err := r.APIReader.Get(ctx, client.ObjectKey{Namespace: cc.Namespace, Name: cc.Spec.SecretName}, &secret)
	if kerrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !metav1.IsControlledBy(&secret, cc) {
		return nil, stalled(reasonSecretConflict, "Secret %q already exists and is not managed by this resource",
			cc.Spec.SecretName)
	}
	return &secret, nil
}

// storedCertificate reads the current certificate from the Secret, with the thumbprint of the one it renewed.
// A missing or unreadable Secret yields neither, so the next certificate replaces every older one at once.
func storedCertificate(secret *corev1.Secret) (*clientcert.Certificate, string) {
	if secret == nil {
		return nil, ""
	}
	current, err := clientcert.Decode(secret.Data[clientPFXKey])
	if err != nil {
		return nil, ""
	}
	return current, secret.Annotations[previousThumbprintAnnotation]
}

// issue creates the certificate that replaces current. A renewal keeps current registered as the previous
// certificate; replacing a lost one keeps nothing.
func (r *RavenDBClientCertificateReconciler) issue(
	cc *ravendbv1.RavenDBClientCertificate, current *clientcert.Certificate,
) (*clientcert.Certificate, string, error) {
	previous := ""
	if current != nil {
		previous = current.Thumbprint()
	}
	next, err := clientcert.Issue(registrationName(cc), cc.Spec.Validity.Duration, r.Now())
	if err != nil {
		return nil, "", err
	}
	r.Recorder.Eventf(cc, corev1.EventTypeNormal, "CertificateIssued",
		"Issued certificate %s, valid until %s", next.Thumbprint(), next.X509.NotAfter.Format(time.RFC3339))
	return next, previous, nil
}

func (r *RavenDBClientCertificateReconciler) syncRegistrations(
	ctx context.Context, rc ravenDB, cc *ravendbv1.RavenDBClientCertificate,
	current *clientcert.Certificate, previous string,
) error {
	desired := ravendb.Certificate{
		Name:              registrationName(cc),
		SecurityClearance: string(cc.Spec.Clearance),
		Permissions:       permissions(cc.Spec.Permissions),
	}
	registered, err := registrations(ctx, rc, cc)
	if err != nil {
		return err
	}

	currentRegistered := false
	for _, reg := range registered {
		isCurrent := strings.EqualFold(reg.Thumbprint, current.Thumbprint())
		if !isCurrent && !strings.EqualFold(reg.Thumbprint, previous) {
			if err := r.revoke(ctx, rc, cc, reg.Thumbprint); err != nil {
				return err
			}
			continue
		}
		currentRegistered = currentRegistered || isCurrent
		if reg.SecurityClearance != desired.SecurityClearance || !maps.Equal(reg.Permissions, desired.Permissions) {
			edited := desired
			edited.Thumbprint = reg.Thumbprint
			if err := rc.EditCertificate(ctx, edited); err != nil {
				return err
			}
		}
	}

	if currentRegistered {
		return nil
	}
	desired.Certificate = current.PublicBase64()
	return rc.RegisterCertificate(ctx, desired)
}

// writeSecret stores the certificate for applications over the Secret read at the start of the reconcile, so a
// Secret someone else created or changed in the meantime fails the write instead of being overwritten. A write
// Kubernetes refuses (quota, admission policy) stalls the resource rather than issuing again every retry.
func (r *RavenDBClientCertificateReconciler) writeSecret(
	ctx context.Context, cc *ravendbv1.RavenDBClientCertificate, existing *corev1.Secret,
	pfx, caPEM []byte, previous string,
) error {
	secret, err := r.desiredSecret(cc, existing, pfx, caPEM, previous)
	if err != nil {
		return err
	}
	switch {
	case existing == nil:
		err = r.Create(ctx, secret)
	case !equality.Semantic.DeepEqual(existing, secret):
		err = r.Update(ctx, secret)
	}
	if kerrors.IsInvalid(err) || kerrors.IsForbidden(err) || kerrors.IsBadRequest(err) {
		return stalled(reasonSecretRejected, "Secret %q cannot be written: %v", cc.Spec.SecretName, err)
	}
	return err
}

// desiredSecret is existing, or a new Secret, holding the certificate. The resource owns it, so Kubernetes
// deletes it together with the resource.
func (r *RavenDBClientCertificateReconciler) desiredSecret(
	cc *ravendbv1.RavenDBClientCertificate, existing *corev1.Secret, pfx, caPEM []byte, previous string,
) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: cc.Namespace, Name: cc.Spec.SecretName},
		Type:       corev1.SecretTypeOpaque,
	}
	if existing != nil {
		secret = existing.DeepCopy()
	}

	secret.Data = map[string][]byte{clientPFXKey: pfx}
	if caPEM != nil {
		secret.Data[caCertKey] = caPEM
	}
	if previous == "" {
		delete(secret.Annotations, previousThumbprintAnnotation)
	} else {
		metav1.SetMetaDataAnnotation(&secret.ObjectMeta, previousThumbprintAnnotation, previous)
	}
	return secret, controllerutil.SetControllerReference(cc, secret, r.Scheme)
}

// finalize revokes every certificate of the resource, then lets Kubernetes remove it and its Secret.
func (r *RavenDBClientCertificateReconciler) finalize(ctx context.Context, cc *ravendbv1.RavenDBClientCertificate) error {
	if err := r.revokeAll(ctx, cc); err != nil {
		return cleanupBlocked(r.Recorder, cc, err)
	}
	return removeFinalizer(ctx, r.Client, cc)
}

func (r *RavenDBClientCertificateReconciler) revokeAll(ctx context.Context, cc *ravendbv1.RavenDBClientCertificate) error {
	rc, err := connectForCleanup(ctx, r.Client, r.Connect, cc.Namespace, cc.Spec.ClusterRef)
	if err != nil || rc == nil {
		return err
	}
	registered, err := registrations(ctx, rc, cc)
	if err != nil {
		return err
	}
	for _, reg := range registered {
		if err := r.revoke(ctx, rc, cc, reg.Thumbprint); err != nil {
			return err
		}
	}
	return nil
}

func (r *RavenDBClientCertificateReconciler) revoke(
	ctx context.Context, rc ravenDB, cc *ravendbv1.RavenDBClientCertificate, thumbprint string,
) error {
	if err := rc.RevokeCertificate(ctx, thumbprint); err != nil {
		return err
	}
	r.Recorder.Eventf(cc, corev1.EventTypeNormal, "CertificateRevoked", "Revoked certificate %s", thumbprint)
	return nil
}

// registrationName identifies the resource's certificates in RavenDB. A namespace holds one cluster, so the
// namespaced name is unique within it.
func registrationName(cc *ravendbv1.RavenDBClientCertificate) string {
	return cc.Namespace + "/" + cc.Name
}

func registrations(
	ctx context.Context, rc ravenDB, cc *ravendbv1.RavenDBClientCertificate,
) ([]ravendb.Certificate, error) {
	all, err := rc.Certificates(ctx)
	if err != nil {
		return nil, err
	}
	var own []ravendb.Certificate
	for _, cert := range all {
		if cert.Name == registrationName(cc) {
			own = append(own, cert)
		}
	}
	return own, nil
}

func permissions(spec map[string]ravendbv1.DatabaseAccess) map[string]string {
	out := make(map[string]string, len(spec))
	for database, access := range spec {
		out[database] = string(access)
	}
	return out
}

func (r *RavenDBClientCertificateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Recorder = mgr.GetEventRecorderFor(common.Manager)
	r.APIReader = mgr.GetAPIReader()
	if r.Connect == nil {
		r.Connect = connectWithAdminCertificate
	}
	if r.Now == nil {
		r.Now = time.Now
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&ravendbv1.RavenDBClientCertificate{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&corev1.Secret{}).
		Watches(&ravendbv1.RavenDBCluster{}, handler.EnqueueRequestsFromMapFunc(
			everyInNamespace(r.Client, func() client.ObjectList { return &ravendbv1.RavenDBClientCertificateList{} }))).
		WithOptions(controller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		Complete(r)
}
