// Copyright © 2019 Banzai Cloud
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package linux

import (
	"fmt"
	"io"
	"os"
	"strings"

	"emperror.dev/errors"
	"github.com/Masterminds/semver"
	"github.com/banzaicloud/pke/cmd/pke/app/util/file"
	"github.com/banzaicloud/pke/cmd/pke/app/util/runner"
)

const (
	dotS                      = "."
	dashS                     = "-"
	cmdYum                    = "/bin/yum"
	cmdRpm                    = "/bin/rpm"
	kubeadm                   = "kubeadm"
	kubectl                   = "kubectl"
	kubelet                   = "kubelet"
	kubernetescni             = "kubernetes-cni"
	disableExcludesKubernetes = "--disableexcludes=kubernetes"
	selinuxConfig             = "/etc/selinux/config"
	banzaiCloudRPMRepo        = "/etc/yum.repos.d/banzaicloud.repo"
	k8sRPMRepoFile            = "/etc/yum.repos.d/kubernetes.repo"
	k8sRPMRepo                = `[kubernetes]
name=Kubernetes
baseurl=https://packages.cloud.google.com/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
exclude=kube*`
)

var (
	errorUnableToParseRPMOutput = errors.New("Unable to parse rpm output")
)

func YumInstall(out io.Writer, packages []string) error {
	_, err := runner.Cmd(out, cmdYum, append([]string{"install", "-y"}, packages...)...).CombinedOutputAsync()
	if err != nil {
		return err
	}

	for _, pkg := range packages {
		if pkg[:1] == "-" {
			continue
		}

		name, ver, rel, arch, err := rpmQuery(out, pkg)
		if err != nil {
			return err
		}
		if name == pkg ||
			name+"-"+ver == pkg ||
			name+"-"+ver+"-"+rel == pkg ||
			name+"-"+ver+"-"+rel+"."+arch == pkg {
			continue
		}
		return errors.New(fmt.Sprintf("expected packgae version after installation: %q, got: %q", pkg, name+"-"+ver+"-"+rel+"."+arch))
	}

	return nil
}

func rpmQuery(out io.Writer, pkg string) (name, version, release, arch string, err error) {
	b, err := runner.Cmd(out, cmdRpm, []string{"-q", pkg}...).Output()
	if err != nil {
		return
	}

	return parseRpmPackageOutput(string(b))
}

func parseRpmPackageOutput(pkg string) (name, version, release, arch string, err error) {
	idx := strings.LastIndex(pkg, dotS)
	if idx < 0 {
		err = errorUnableToParseRPMOutput
		return
	}
	arch = pkg[idx+1:]

	pkg = pkg[:idx]
	idx = strings.LastIndex(pkg, dashS)
	if idx < 0 {
		err = errorUnableToParseRPMOutput
		return
	}
	release = pkg[idx+1:]

	pkg = pkg[:idx]
	idx = strings.LastIndex(pkg, dashS)
	if idx < 0 {
		err = errorUnableToParseRPMOutput
		return
	}
	version = pkg[idx+1:]
	name = pkg[:idx]

	return
}

var _ ContainerdPackages = (*YumInstaller)(nil)
var _ KubernetesPackages = (*YumInstaller)(nil)

type YumInstaller struct{}

func (y *YumInstaller) InstallKubernetesPrerequisites(out io.Writer, kubernetesVersion string) error {
	// Set SELinux in permissive mode (effectively disabling it)
	// setenforce 0
	err := runner.Cmd(out, "setenforce", "0").Run()
	if err != nil {
		return err
	}
	// sed -i 's/^SELINUX=enforcing$/SELINUX=permissive/' /etc/selinux/config
	err = runner.Cmd(out, "sed", "-i", "s/^SELINUX=enforcing$/SELINUX=permissive/", selinuxConfig).Run()
	if err != nil {
		return err
	}

	if err = SwapOff(out); err != nil {
		return err
	}

	if err := ModprobeKubeProxyIPVSModules(out); err != nil {
		return err
	}

	if err := SysctlLoadAllFiles(out); err != nil {
		return errors.Wrapf(err, "unable to load all sysctl rules from files")
	}

	if _, err := os.Stat(banzaiCloudRPMRepo); err != nil {
		// Add kubernetes repo
		// cat <<EOF > /etc/yum.repos.d/kubernetes.repo
		// [kubernetes]
		// name=Kubernetes
		// baseurl=https://packages.cloud.google.com/yum/repos/kubernetes-el7-x86_64
		// enabled=1
		// gpgcheck=1
		// repo_gpgcheck=1
		// gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
		// exclude=kube*
		// EOF
		err = file.Overwrite(k8sRPMRepoFile, k8sRPMRepo)
		if err != nil {
			return err
		}
	}
	return nil
}

func NewYumInstaller() *YumInstaller {
	return &YumInstaller{}
}

func (y *YumInstaller) InstallKubernetesPackages(out io.Writer, kubernetesVersion string) error {
	// yum install -y kubelet kubeadm kubectl --disableexcludes=kubernetes
	p := []string{
		mapYumPackageVersion(kubelet, kubernetesVersion),
		mapYumPackageVersion(kubeadm, kubernetesVersion),
		mapYumPackageVersion(kubectl, kubernetesVersion),
		mapYumPackageVersion(kubernetescni, kubernetesVersion),
		disableExcludesKubernetes,
	}

	return YumInstall(out, p)
}

func (y *YumInstaller) InstallKubeadmPackage(out io.Writer, kubernetesVersion string) error {
	// yum install -y kubeadm --disableexcludes=kubernetes
	pkg := []string{
		mapYumPackageVersion(kubeadm, kubernetesVersion),
		mapYumPackageVersion(kubelet, kubernetesVersion),       // kubeadm dependency
		mapYumPackageVersion(kubernetescni, kubernetesVersion), // kubeadm dependency
		disableExcludesKubernetes,
	}
	return YumInstall(out, pkg)
}

func (y *YumInstaller) InstallContainerdPrerequisites(out io.Writer, containerdVersion string) error {
	// yum install -y libseccomp
	if err := YumInstall(out, []string{"libseccomp"}); err != nil {
		return errors.Wrap(err, "unable to install libseccomp package")
	}

	return nil
}

func mapYumPackageVersion(pkg, kubernetesVersion string) string {
	switch pkg {
	case kubeadm:
		return "kubeadm-" + kubernetesVersion + "-0"

	case kubectl:
		return "kubectl-" + kubernetesVersion + "-0"

	case kubelet:
		return "kubelet-" + kubernetesVersion + "-0"

	case kubernetescni:
		ver, _ := semver.NewVersion(kubernetesVersion)
		c, _ := semver.NewConstraint(">=1.12.7,<1.13.x || >=1.13.5")
		if c.Check(ver) {
			return "kubernetes-cni-0.7.5-0"
		}
		return "kubernetes-cni-0.6.0-0"

	default:
		return ""
	}
}
