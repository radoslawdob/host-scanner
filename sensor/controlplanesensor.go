package sensor

import (
	"errors"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

const (
	apiServerExe                   = "/kube-apiserver"
	controllerManagerExe           = "/kube-controller-manager"
	schedulerExe                   = "/kube-scheduler"
	etcdExe                        = "/etcd"
	etcdDataDirArg                 = "--data-dir"
	apiEncryptionProviderConfigArg = "--encryption-provider-config"

	// Default files paths according to https://workbench.cisecurity.org/benchmarks/8973/sections/1126652
	apiServerSpecsPath          = "/etc/kubernetes/manifests/kube-apiserver.yaml"
	controllerManagerSpecsPath  = "/etc/kubernetes/manifests/kube-controller-manager.yaml"
	controllerManagerConfigPath = "/etc/kubernetes/controller-manager.conf"
	schedulerSpecsPath          = "/etc/kubernetes/manifests/kube-scheduler.yaml"
	schedulerConfigPath         = "/etc/kubernetes/scheduler.conf"
	etcdConfigPath              = "/etc/kubernetes/manifests/etcd.yaml"
	adminConfigPath             = "/etc/kubernetes/admin.conf"
	pkiDir                      = "/etc/kubernetes/pki"

	// TODO: cni
)

var (
	ErrDataDirNotFound = errors.New("failed to find etcd data-dir")
)

// KubeProxyInfo holds information about kube-proxy process
type ControlPlaneInfo struct {
	APIServerInfo         *ApiServerInfo  `json:"APIServerInfo,omitempty"`
	ControllerManagerInfo *K8sProcessInfo `json:"controllerManagerInfo,omitempty"`
	SchedulerInfo         *K8sProcessInfo `json:"schedulerInfo,omitempty"`
	EtcdConfigFile        *FileInfo       `json:"etcdConfigFile,omitempty"`
	EtcdDataDir           *FileInfo       `json:"etcdDataDir,omitempty"`
	AdminConfigFile       *FileInfo       `json:"adminConfigFile,omitempty"`
	PKIDIr                *FileInfo       `json:"PKIDir,omitempty"`
	PKIFiles              []*FileInfo     `json:"PKIFiles,omitempty"`
	CNIConfigFiles        []*FileInfo     `json:"CNIConfigFiles"`
	CNIConfigPath         string          `json:"CNIConfigPath,omitempty"`
}

// K8sProcessInfo holds information about a k8s process
type K8sProcessInfo struct {
	// Information about the process specs file (if relevant)
	SpecsFile *FileInfo `json:"specsFile,omitempty"`

	// Information about the process config file (if relevant)
	ConfigFile *FileInfo `json:"configFile,omitempty"`

	// Information about the process kubeconfig file (if relevant)
	KubeConfigFile *FileInfo `json:"kubeConfigFile,omitempty"`

	// Information about the process client ca file (if relevant)
	ClientCAFile *FileInfo `json:"clientCAFile,omitempty"`

	// Raw cmd line of the process
	CmdLine string `json:"cmdLine"`
}

type ApiServerInfo struct {
	EncryptionProviderConfigFile *FileInfo `json:"encryptionProviderConfigFile,omitempty"`
	*K8sProcessInfo              `json:",inline"`
}

// getEtcdDataDir find the `data-dir` path of etcd k8s component
func getEtcdDataDir() (string, error) {

	proc, err := LocateProcessByExecSuffix(etcdExe)
	if err != nil {
		return "", fmt.Errorf("failed to locate kube-proxy process: %w", err)
	}

	dataDir, ok := proc.GetArg(etcdDataDirArg)
	if !ok || dataDir == "" {
		return "", ErrDataDirNotFound
	}

	return dataDir, nil
}

func makeProcessInfoVerbose(p *ProcessDetails, specsPath, configPath, kubeConfigPath, clientCaPath string) *K8sProcessInfo {
	ret := K8sProcessInfo{}

	// init files
	files := []struct {
		data **FileInfo
		path string
		file string
	}{
		{&ret.SpecsFile, specsPath, "specs"},
		{&ret.ConfigFile, configPath, "config"},
		{&ret.KubeConfigFile, kubeConfigPath, "kubeconfig"},
		{&ret.ClientCAFile, clientCaPath, "calient ca certificate"},
	}

	// get data
	for i := range files {
		file := &files[i]
		if file.path == "" {
			continue
		}

		*file.data = makeHostFileInfoVerbose(file.path, false,
			zap.String("in", "makeProcessInfoVerbose"),
			zap.String("path", file.path),
		)
	}

	if p != nil {
		ret.CmdLine = p.RawCmd()
	}

	// Return `nil` if wasn't able to find any data
	if ret == (K8sProcessInfo{}) {
		return nil
	}

	return &ret
}

// makeAPIserverEncryptionProviderConfigFile returns a FileInfo object for the encryption provider config file of the API server. Required for https://workbench.cisecurity.org/sections/1126663/recommendations/1838675
func makeAPIserverEncryptionProviderConfigFile(p *ProcessDetails) *FileInfo {
	encryptionProviderConfigPath, ok := p.GetArg(apiEncryptionProviderConfigArg)
	if !ok {
		zap.L().Warn("failed to find encryption provider config path", zap.String("in", "makeAPIserverEncryptionProviderConfigFile"))
		return nil
	}

	fi, err := makeContaineredFileInfo(encryptionProviderConfigPath, true, p)
	if err != nil {
		zap.L().Warn("failed to create encryption provider config file info", zap.Error(err))
		return nil
	}
	return fi
}

// SenseControlPlaneInfo return `ControlPlaneInfo`
func SenseControlPlaneInfo() (*ControlPlaneInfo, error) {
	var err error
	ret := ControlPlaneInfo{}

	debugInfo := zap.String("in", "SenseControlPlaneInfo")

	apiProc, err := LocateProcessByExecSuffix(apiServerExe)
	if err != nil {
		zap.L().Error("SenseControlPlaneInfo", zap.Error(err))
	}

	controllerMangerProc, err := LocateProcessByExecSuffix(controllerManagerExe)
	if err != nil {
		zap.L().Error("SenseControlPlaneInfo", zap.Error(err))
	}

	SchedulerProc, err := LocateProcessByExecSuffix(schedulerExe)
	if err != nil {
		zap.L().Error("SenseControlPlaneInfo", zap.Error(err))
	}

	ret.APIServerInfo = &ApiServerInfo{}
	ret.APIServerInfo.K8sProcessInfo = makeProcessInfoVerbose(apiProc, apiServerSpecsPath, "", "", "")
	ret.APIServerInfo.EncryptionProviderConfigFile = makeAPIserverEncryptionProviderConfigFile(apiProc)
	ret.ControllerManagerInfo = makeProcessInfoVerbose(controllerMangerProc, controllerManagerSpecsPath, controllerManagerConfigPath, "", "")
	ret.SchedulerInfo = makeProcessInfoVerbose(SchedulerProc, schedulerSpecsPath, schedulerConfigPath, "", "")

	// EtcdConfigFile
	ret.EtcdConfigFile = makeHostFileInfoVerbose(etcdConfigPath,
		false,
		debugInfo,
		zap.String("component", "EtcdConfigFile"),
	)

	// AdminConfigFile
	ret.AdminConfigFile = makeHostFileInfoVerbose(adminConfigPath,
		false,
		debugInfo,
		zap.String("component", "AdminConfigFile"),
	)

	// PKIDIr
	ret.PKIDIr = makeHostFileInfoVerbose(pkiDir,
		false,
		debugInfo,
		zap.String("component", "PKIDIr"),
	)

	// PKIFiles
	ret.PKIFiles, err = makeHostDirFilesInfo(pkiDir, true, nil, 0)
	if err != nil {
		zap.L().Error("SenseControlPlaneInfo failed to get PKIFiles info", zap.Error(err))
	}

	// etcd data-dir
	etcdDataDir, err := getEtcdDataDir()
	if err != nil {
		zap.L().Error("SenseControlPlaneInfo", zap.Error(ErrDataDirNotFound))
	} else {
		ret.EtcdDataDir = makeHostFileInfoVerbose(etcdDataDir,
			false,
			debugInfo,
			zap.String("component", "EtcdDataDir"),
		)
	}

	// *** Start handling CNI Files
	cni_paths := getContainerRuntimeCNIPaths()

	if cni_paths == nil {
		zap.L().Error("SenseControlPlaneInfo Failed to get CNI paths")
	} else {

		//Getting CNI config files
		CNIConfigInfo, err := makeHostDirFilesInfo(cni_paths.Conf_dir, true, nil, 0)
		ret.CNIConfigFiles = CNIConfigInfo
		ret.CNIConfigPath = cni_paths.Conf_dir

		if err != nil {
			zap.L().Debug("SenseControlPlaneInfo failed to  makeHostDirFilesInfo for CNI Config files",
				zap.String("path", cni_paths.Conf_dir),
				zap.Error(err),
			)
		} else {
			if len(CNIConfigInfo) == 0 {
				zap.L().Debug("SenseControlPlaneInfo - no cni config files were found.",
					zap.String("path", cni_paths.Conf_dir),
					zap.Error(err),
				)
			}
		}

	}

	// If wasn't able to find any data - this is not a control plane
	if ret.APIServerInfo.K8sProcessInfo == nil &&
		ret.ControllerManagerInfo == nil &&
		ret.SchedulerInfo == nil &&
		ret.EtcdConfigFile == nil &&
		ret.EtcdDataDir == nil &&
		ret.AdminConfigFile == nil &&
		ret.PKIDIr == nil &&
		ret.PKIFiles == nil &&
		ret.CNIConfigFiles == nil {
		return nil, &SenseError{
			Massage:  "not a control plane node",
			Function: "SenseControlPlaneInfo",
			Code:     http.StatusNotFound,
		}
	}

	return &ret, nil
}
