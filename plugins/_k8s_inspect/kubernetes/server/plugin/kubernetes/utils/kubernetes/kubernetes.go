package kubernetes

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/flipped-aurora/gin-vue-admin/server/global"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/model"
	"github.com/flipped-aurora/gin-vue-admin/server/plugin/kubernetes/utils/collectons"
	"go.uber.org/zap"
	authV1 "k8s.io/api/authorization/v1"
	v1 "k8s.io/api/authorization/v1"
	certv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	rbacV1 "k8s.io/api/rbac/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Interface interface {
	Config() (*rest.Config, error)
	Client() (*kubernetes.Clientset, error)
	DynamicClient() (dynamic.Interface, error)
	VersionMinor() (int, error)
	IsNamespacedResource(resourceName string) (bool, error)
	GetUserNamespaceNames(username string) ([]string, error)
	GetKubeCaCrt() (string, error)
	CreateClientCertificate(username string) (string, string, error)
	HasPermission(attributes v1.ResourceAttributes) (model.PermissionCheckResult, error)
	CheckRequiredPermissionsclient(requiredPermissions map[string][]string) (string, error)
	CheckPermissions() (err error)
	CreateDefaultClusterRoles() (err error)
	CleanAllRBACResource() (err error)
	ClusterRoles(roleType string) (roles []rbacv1.ClusterRole, err error)
	ServerGroupsAndResources(api_type string) (groups []model.ApiGroupOption, err error)
	CreateOrUpdateClusterRoleBinding(clusterRoleName string, username string, builtIn bool) error
	CreateOrUpdateRolebinding(namespace string, clusterRoleName string, username string, builtIn bool) error
	CleanManagedClusterRoleBinding(username string) error
	CleanManagedRoleBinding(username string) error
}

type Kubernetes struct {
	*model.Cluster
	*model.User
}

func NewKubernetes(cluster *model.Cluster, user *model.User) Interface {
	return &Kubernetes{cluster, user}
}

func (k *Kubernetes) Config() (config *rest.Config, err error) {
	if k.KubeType == 1 {
		config, err = clientcmd.RESTConfigFromKubeConfig([]byte(k.Cluster.KubeConfig))
		if err != nil {
			global.GVA_LOG.Error("RESTConfigFromKubeConfig init failed: ", zap.Error(err))
			return nil, err
		}

		// 在请求中添加伪装用户身份
		if k.Username != "" {
			config.Impersonate = rest.ImpersonationConfig{
				UserName: k.Username,
			}
		}

		return config, err
	}

	if k.KubeType == 2 {
		config = &rest.Config{
			Host:            k.ApiAddress,
			BearerToken:     k.Cluster.KubeConfig,
			TLSClientConfig: rest.TLSClientConfig{Insecure: true},
		}

		// 在请求中添加伪装用户身份
		if k.Username != "" {
			config.Impersonate = rest.ImpersonationConfig{
				UserName: k.Username,
			}
		}

		return config, err
	}

	return
}

func (k *Kubernetes) VersionMinor() (int, error) {
	v, err := k.Version()
	if err != nil {
		return 0, err
	}
	reg := regexp.MustCompile("[^0-9]")
	minor, err := strconv.Atoi(reg.ReplaceAllString(v.Minor, ""))
	return minor, err
}

func (k *Kubernetes) Version() (*version.Info, error) {
	client, err := k.Client()
	if err != nil {
		return nil, err
	}
	return client.ServerVersion()
}

func (k *Kubernetes) Client() (*kubernetes.Clientset, error) {
	cfg, err := k.Config()
	if err != nil {
		global.GVA_LOG.Error("Config init failed: ", zap.Error(err))
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		global.GVA_LOG.Error("clientset init failed: ", zap.Error(err))
		return nil, err
	}

	return clientset, err
}

func (k *Kubernetes) DynamicClient() (dynamic.Interface, error) {
	cfg, err := k.Config()
	if err != nil {
		global.GVA_LOG.Error("Config init failed: ", zap.Error(err))
		return nil, err
	}

	clientset, err := dynamic.NewForConfig(cfg)
	if err != nil {
		global.GVA_LOG.Error("clientset init failed: ", zap.Error(err))
		return nil, err
	}

	return clientset, err
}

func (k *Kubernetes) CanVisitAllNamespace(username string) (bool, error) {
	client, err := k.Client()
	if err != nil {
		return false, err
	}
	roleSet := collectons.NewStringSet()
	labels := []string{
		fmt.Sprintf("%s=%s", LabelManageKey, "devops"),
		fmt.Sprintf("%s=%s", LabelClusterId, k.Cluster.UUID),
		fmt.Sprintf("%s=%s", LabelUsername, username),
	}
	clusterrolebindings, err := client.RbacV1().ClusterRoleBindings().List(context.TODO(), metav1.ListOptions{
		LabelSelector: strings.Join(labels, ","),
	})
	if err != nil {
		return false, err
	}

	for i := range clusterrolebindings.Items {
		roleSet.Add(clusterrolebindings.Items[i].RoleRef.Name)
	}
	for _, roleName := range roleSet.ToSlice() {
		role, err := client.RbacV1().ClusterRoles().Get(context.TODO(), roleName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for i := range role.Rules {
			if collectons.IndexOfStringSlice(role.Rules[i].APIGroups, "*") != -1 && collectons.IndexOfStringSlice(role.Rules[i].Resources, "*") != -1 {
				return true, nil
			}
		}
	}
	return false, nil
}

func (k *Kubernetes) GetUserNamespaceNames(username string) ([]string, error) {

	client, err := k.Client()
	if err != nil {
		return nil, err
	}

	all, err := k.CanVisitAllNamespace(username)
	if err != nil {
		return nil, err
	}

	namespaceSet := collectons.NewStringSet()
	if all {
		ns, err := client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for i := range ns.Items {
			if ns.Items[i].Status.Phase == "Active" {
				namespaceSet.Add(ns.Items[i].Name)
			}
		}
	} else {
		rbs, err := client.RbacV1().RoleBindings("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for i := range rbs.Items {
			for j := range rbs.Items[i].Subjects {
				if rbs.Items[i].Subjects[j].Kind == "User" && rbs.Items[i].Subjects[j].Name == username {
					namespaceSet.Add(rbs.Items[i].Namespace)
				}
			}
		}
	}

	result := namespaceSet.ToSlice()
	sort.Strings(result)
	return result, nil
}

func (k *Kubernetes) IsNamespacedResource(resourceName string) (bool, error) {
	if resourceName == "events" || resourceName == "nodes" {
		return false, nil
	}
	client, err := k.Client()
	if err != nil {
		return false, err
	}
	apiList, err := client.ServerPreferredNamespacedResources()
	if err != nil && len(apiList) == 0 {
		return false, err
	}
	for i := range apiList {
		for j := range apiList[i].APIResources {
			if apiList[i].APIResources[j].Name == resourceName {
				return true, nil
			}
		}
	}
	return false, nil
}

func (k *Kubernetes) GetKubeCaCrt() (cacrt string, err error) {
	clientset, err := k.Client()
	if err != nil {
		return "", err
	}

	User := fmt.Sprintf("%s-%s-%d", "random", "devops", time.Now().Unix())

	serviceaccount := &corev1.ServiceAccount{TypeMeta: metav1.TypeMeta{}, ObjectMeta: metav1.ObjectMeta{Name: User}}
	if _, err = clientset.CoreV1().ServiceAccounts("default").Create(context.TODO(), serviceaccount, metav1.CreateOptions{}); err != nil {
		return "", errors.New("ServiceAccounts 创建失败: " + err.Error())
	}

	secret := &corev1.Secret{TypeMeta: metav1.TypeMeta{}, ObjectMeta: metav1.ObjectMeta{Name: User,
		Annotations: map[string]string{"kubernetes.io/service-account.name": User}},
		Type: "kubernetes.io/service-account-token",
	}
	if _, err = clientset.CoreV1().Secrets("default").Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
		return "", errors.New("Secrets 创建失败: " + err.Error())
	}

	for i := 1; i <= 150; i++ {
		time.Sleep(3 * time.Second)

		secrets, err := clientset.CoreV1().Secrets("default").Get(context.TODO(), User, metav1.GetOptions{})
		if err != nil {
			continue
		}

		secretCaCrt := fmt.Sprintf("%s", secrets.Data["ca.crt"])
		if secretCaCrt != "" {
			if err = clientset.CoreV1().Secrets("default").Delete(context.TODO(), User, metav1.DeleteOptions{}); err != nil {
				return "", err
			}

			if err = clientset.CoreV1().ServiceAccounts("default").Delete(context.TODO(), User, metav1.DeleteOptions{}); err != nil {
				return "", err
			}
		}

		return base64.StdEncoding.EncodeToString([]byte(secretCaCrt)), err
	}

	return "", err
}

func (k *Kubernetes) CreateClientCertificate(username string) (privateKey, publicCert string, err error) {
	clientset, err := k.Client()
	if err != nil {
		return privateKey, publicCert, err
	}

	// 生成一个私钥
	RandprivateKey, err := GeneratePrivateKey()
	if err != nil {
		return privateKey, publicCert, err
	}

	// 生成用户证书申请
	cert, err := CreateClientCertificateRequest(username, RandprivateKey)
	if err != nil {
		return privateKey, publicCert, err
	}

	csr := certv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: username,
		},
		Spec: certv1.CertificateSigningRequestSpec{
			SignerName: "kubernetes.io/kube-apiserver-client",
			Request:    cert,
			Groups: []string{
				"system:authenticated",
			},
			Usages: []certv1.KeyUsage{
				"client auth",
			},
		},
	}

	createResp, err := clientset.CertificatesV1().CertificateSigningRequests().Create(context.TODO(), &csr, metav1.CreateOptions{})
	if err != nil {
		return privateKey, publicCert, err
	}

	// 审批证书
	createResp.Status.Conditions = append(createResp.Status.Conditions, certv1.CertificateSigningRequestCondition{
		Reason:         "Approved by Devops",
		Type:           certv1.CertificateApproved,
		LastUpdateTime: metav1.Now(),
		Status:         "True",
	})

	updateResp, err := clientset.CertificatesV1().CertificateSigningRequests().UpdateApproval(context.TODO(), createResp.Name, createResp, metav1.UpdateOptions{})
	if err != nil {
		return privateKey, publicCert, err
	}

	for {
		max_num := 0
		for {
			time.Sleep(3 * time.Second)
			if max_num > 150 {
				break
			}

			getResp, err := clientset.CertificatesV1().CertificateSigningRequests().Get(context.TODO(), updateResp.Name, metav1.GetOptions{})
			if err != nil {
				max_num += 1
				global.GVA_LOG.Warn("CertificateSigningRequests Get failed: " + err.Error())
				continue
			}

			if getResp.Status.Certificate != nil {
				if err = clientset.CertificatesV1().CertificateSigningRequests().Delete(context.TODO(), username, metav1.DeleteOptions{}); err != nil {
					max_num += 1
					global.GVA_LOG.Warn("CertificateSigningRequests Get failed: " + err.Error())
					continue
				}

				return base64.StdEncoding.EncodeToString(
					pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: RandprivateKey}),
				), base64.StdEncoding.EncodeToString(getResp.Status.Certificate), err
			}

			max_num += 1
			continue
		}
	}
}

func (k *Kubernetes) HasPermission(attributes v1.ResourceAttributes) (model.PermissionCheckResult, error) {
	clientset, err := k.Client()
	if err != nil {
		global.GVA_LOG.Error("clientset init failed: " + err.Error())
		return model.PermissionCheckResult{}, err
	}

	resp, err := clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(context.TODO(), &v1.SelfSubjectAccessReview{
		Spec: v1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &attributes,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return model.PermissionCheckResult{}, err
	}

	return model.PermissionCheckResult{
		Resource: attributes,
		Allowed:  resp.Status.Allowed,
	}, nil

}

func (k *Kubernetes) CheckRequiredPermissionsclient(requiredPermissions map[string][]string) (string, error) {
	wg := sync.WaitGroup{}
	errCh := make(chan error)
	resultCh := make(chan model.PermissionCheckResult)
	doneCh := make(chan struct{})
	for key := range requiredPermissions {
		for i := range requiredPermissions[key] {
			wg.Add(1)
			i := i
			go func(key string, index int) {
				rs, err := k.HasPermission(authV1.ResourceAttributes{
					Verb:     requiredPermissions[key][i],
					Resource: key,
				})
				if err != nil {
					errCh <- err
					wg.Done()
					return
				}
				resultCh <- rs
				wg.Done()
			}(key, i)
		}
	}
	go func() {
		wg.Wait()
		doneCh <- struct{}{}
	}()
	for {
		select {
		case <-doneCh:
			goto end
		case err := <-errCh:
			return "", err
		case b := <-resultCh:
			if !b.Allowed {
				return b.Resource.Resource, nil
			}
		}
	}
end:
	return "", nil
}

func (k *Kubernetes) CheckPermissions() (err error) {
	permissions := map[string][]string{
		"namespaces":       {"get", "post", "delete"},
		"clusterroles":     {"get", "post", "delete"},
		"clusterrolebings": {"get", "post", "delete"},
		"roles":            {"get", "post", "delete"},
		"rolebindings":     {"get", "post", "delete"},
	}

	notAllowed, err := k.CheckRequiredPermissionsclient(permissions)
	if err != nil {
		global.GVA_LOG.Error(fmt.Sprintf("notAllowed faile: %s", err.Error()))
		return errors.New(fmt.Sprintf("notAllowed faile: %s", err.Error()))
	}

	if notAllowed != "" {
		return errors.New(fmt.Sprintf("permission %s required", notAllowed))
	}

	return err

}

func (k *Kubernetes) CreateDefaultClusterRoles() error {
	clientset, err := k.Client()
	if err != nil {
		global.GVA_LOG.Error("clientset init failed: " + err.Error())
		return err
	}

	for i := range InitClusterRoles {
		instance, err := clientset.RbacV1().ClusterRoles().Get(context.TODO(), InitClusterRoles[i].Name, metav1.GetOptions{})
		if err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "not found") {
				return err
			}
		}

		// If it already exists, try updating
		if instance == nil || instance.Name == "" {
			_, err = clientset.RbacV1().ClusterRoles().Create(context.TODO(), &InitClusterRoles[i], metav1.CreateOptions{})
			if err != nil {
				return err
			}
		} else {
			_, err = clientset.RbacV1().ClusterRoles().Update(context.TODO(), &InitClusterRoles[i], metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (k *Kubernetes) CleanManagedClusterRoleBinding(username string) error {
	client, err := k.Client()
	if err != nil {
		return err
	}
	labels := []string{
		fmt.Sprintf("%s=%s", LabelManageKey, "devops"),
		fmt.Sprintf("%s=%s", LabelClusterId, k.Cluster.UUID),
	}
	if username != "" {
		labels = append(labels, fmt.Sprintf("%s=%s", LabelUsername, username))
	}
	return client.RbacV1().ClusterRoleBindings().DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: strings.Join(labels, ","),
	})
}

func (k *Kubernetes) CleanManagedClusterRole() error {
	client, err := k.Client()
	if err != nil {
		return err
	}

	labels := []string{
		fmt.Sprintf("%s=%s", LabelManageKey, "devops"),
	}
	return client.RbacV1().ClusterRoles().DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: strings.Join(labels, ","),
	})
}

func (k *Kubernetes) CleanManagedRoleBinding(username string) error {
	client, err := k.Client()
	if err != nil {
		return err
	}
	nss, err := client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	labels := []string{
		fmt.Sprintf("%s=%s", LabelManageKey, "devops"),
	}
	if username != "" {
		labels = append(labels, fmt.Sprintf("%s=%s", LabelUsername, username))
	}
	for i := range nss.Items {
		if err := client.RbacV1().RoleBindings(nss.Items[i].Name).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: strings.Join(labels, ","),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (k *Kubernetes) CleanAllRBACResource() error {
	if err := k.CleanManagedClusterRole(); err != nil {
		return err
	}

	if err := k.CleanManagedClusterRoleBinding(""); err != nil {
		return err
	}

	if err := k.CleanManagedRoleBinding(""); err != nil {
		return err
	}

	return nil
}

func (k *Kubernetes) ClusterRoles(roleType string) (roles []rbacv1.ClusterRole, err error) {
	client, err := k.Client()
	if err != nil {
		return nil, err
	}

	labels := []string{
		fmt.Sprintf("%s=%s", LabelManageKey, "devops"),
		fmt.Sprintf("%s=%s", "devops/role-type", roleType),
	}

	roleList, err := client.RbacV1().ClusterRoles().List(context.TODO(), metav1.ListOptions{
		LabelSelector: strings.Join(labels, ","),
	})

	if err != nil {
		return nil, err
	}

	return roleList.Items, err

}

func (k *Kubernetes) ServerGroupsAndResources(api_type string) (groups []model.ApiGroupOption, err error) {
	client, err := k.Client()
	if err != nil {
		return nil, err
	}

	_, rss, err := client.ServerGroupsAndResources()
	if err != nil {
		return nil, err
	}

	var (
		groupMap = make(map[string]struct{})
	)

	itemResource := model.ApiResourceOption{
		Resource: "*",
		Verbs:    []string{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"},
	}

	if api_type == "namespace" {
		groups = append(groups, model.ApiGroupOption{Group: "*", Resources: []model.ApiResourceOption{itemResource}})
	} else {
		groups = append(groups, model.ApiGroupOption{Group: "*", Resources: []model.ApiResourceOption{itemResource}})
	}

	for _, group := range rss {
		if strings.Contains(group.GroupVersion, "/") {
			group.GroupVersion = group.GroupVersion[0:strings.Index(group.GroupVersion, "/")]
		}

		name := group.GroupVersion
		if name == "v1" {
			name = ""
		}

		if _, ok := groupMap[name]; ok {
			continue
		}

		groupMap[name] = struct{}{}
		itemGroup := model.ApiGroupOption{Group: name, Resources: []model.ApiResourceOption{itemResource}}
		for _, resource := range group.APIResources {
			if !(api_type == "namespace" && resource.Namespaced) && !(api_type == "cluster" && !resource.Namespaced) {
				continue
			}
			itemGroup.Resources = append(itemGroup.Resources, model.ApiResourceOption{Resource: resource.Name, Verbs: resource.Verbs})
		}

		if len(itemGroup.Resources) > 1 {
			groups = append(groups, itemGroup)
		}

	}

	return groups, err

}

func (k *Kubernetes) CreateOrUpdateClusterRoleBinding(clusterRoleName string, username string, builtIn bool) error {
	client, err := k.Client()
	if err != nil {
		return err
	}

	name := fmt.Sprintf("%s:%s:%s", username, clusterRoleName, k.Cluster.UUID.String())
	labels := map[string]string{
		LabelManageKey: "devops",
		LabelClusterId: k.Cluster.UUID.String(),
		LabelUsername:  username,
	}
	annotations := map[string]string{
		"built-in":   strconv.FormatBool(builtIn),
		"created-at": time.Now().Format("2006-01-02 15:04:05"),
	}
	item := rbacV1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
		Subjects: []rbacV1.Subject{
			{
				Kind: "User",
				Name: username,
			},
		},
		RoleRef: rbacV1.RoleRef{
			Kind: "ClusterRole",
			Name: clusterRoleName,
		},
	}
	baseItem, err := client.RbacV1().ClusterRoleBindings().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
	}

	if baseItem != nil && baseItem.Name != "" {
		_, err := client.RbacV1().ClusterRoleBindings().Create(context.TODO(), &item, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else {
		_, err := client.RbacV1().ClusterRoleBindings().Update(context.TODO(), &item, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (k *Kubernetes) CreateOrUpdateRolebinding(namespace string, clusterRoleName string, username string, builtIn bool) error {
	client, err := k.Client()
	if err != nil {
		return err
	}
	labels := map[string]string{
		LabelManageKey: "devops",
		LabelClusterId: k.Cluster.UUID.String(),
		LabelUsername:  username,
	}
	annotations := map[string]string{
		"built-in":   strconv.FormatBool(builtIn),
		"created-at": time.Now().Format("2006-01-02 15:04:05"),
	}
	name := fmt.Sprintf("%s:%s:%s:%s", namespace, username, clusterRoleName, k.Cluster.UUID)
	item := rbacV1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
			Namespace:   namespace,
		},
		Subjects: []rbacV1.Subject{
			{
				Kind: "User",
				Name: username,
			},
		},
		RoleRef: rbacV1.RoleRef{
			Kind: "ClusterRole",
			Name: clusterRoleName,
		},
	}
	baseItem, err := client.RbacV1().RoleBindings(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return err
		}
	}
	if baseItem != nil && baseItem.Name != "" {
		_, err := client.RbacV1().RoleBindings(namespace).Create(context.TODO(), &item, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	} else {
		_, err := client.RbacV1().RoleBindings(namespace).Update(context.TODO(), &item, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func GenerateTLSTransport(c *model.Cluster, u *model.User) (http.RoundTripper, error) {
	kube := NewKubernetes(c, u)
	kubeconfig, err := kube.Config()
	if err != nil {
		return nil, err
	}

	return rest.TransportFor(kubeconfig)
}

func CreateClientCertificateRequest(userName string, key []byte, org ...string) ([]byte, error) {
	privateKey, err := x509.ParsePKCS1PrivateKey(key)
	if err != nil {
		return nil, err
	}

	subj := pkix.Name{
		CommonName: userName,
	}
	for i := range org {
		subj.Organization = append(subj.Organization, org[i])
	}
	rawSubj := subj.ToRDNSequence()
	asn1Subj, err := asn1.Marshal(rawSubj)
	if err != nil {
		return nil, err
	}

	csr := &x509.CertificateRequest{
		RawSubject:         asn1Subj,
		SignatureAlgorithm: x509.SHA256WithRSA,
	}
	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, csr, privateKey)
	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes}), nil
}

func GeneratePrivateKey() ([]byte, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	return x509.MarshalPKCS1PrivateKey(privateKey), nil
}
