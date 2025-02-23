package sensor

import (
	"fmt"

	"go.uber.org/zap"
	"sigs.k8s.io/yaml"
)

const (
	procDirName            = "/proc"
	kubeletProcessSuffix   = "/kubelet"
	kubeletConfigArgName   = "--config"
	kubeletClientCAArgName = "--client-ca-file"

	// Default paths
	kubeletConfigDefaultPath     = "/var/lib/kubelet/config.yaml"
	kubeletKubeConfigDefaultPath = "/etc/kubernetes/kubelet.conf"
)

// KubeletInfo holds information about kubelet
type KubeletInfo struct {
	// ServiceFile is a list of files used to configure the kubelet service.
	// Most of the times it will be a single file, under /etc/systemd/system/kubelet.service.d.
	ServiceFiles []FileInfo `json:"serviceFiles,omitempty"`

	// Information about kubelete config file
	ConfigFile *FileInfo `json:"configFile,omitempty"`

	// Information about the kubeconfig file of kubelet
	KubeConfigFile *FileInfo `json:"kubeConfigFile,omitempty"`

	// Information about the client ca file of kubelet (if exist)
	ClientCAFile *FileInfo `json:"clientCAFile,omitempty"`

	// Raw cmd line of kubelet process
	CmdLine string `json:"cmdLine"`
}

func LocateKubeletProcess() (*ProcessDetails, error) {
	return LocateProcessByExecSuffix(kubeletProcessSuffix)
}

func ReadKubeletConfig(kubeletConfArgs string) ([]byte, error) {
	conte, err := ReadFileOnHostFileSystem(kubeletConfArgs)
	zap.L().Debug("raw content", zap.ByteString("cont", conte))
	return conte, err
}

func makeKubeletServiceFilesInfo(pid int) []FileInfo {
	files, err := getKubeletServiceFiles(pid)
	if err != nil {
		zap.L().Warn("failed to getKubeletServiceFiles", zap.Error(err))
		return nil
	}

	serviceFiles := []FileInfo{}
	for _, file := range files {
		info := makeHostFileInfoVerbose(file, false, zap.String("in", "makeProcessInfoVerbose"))
		if info != nil {
			serviceFiles = append(serviceFiles, *info)
		}
	}

	if len(serviceFiles) == 0 {
		return nil
	}

	return serviceFiles
}

// SenseKubeletInfo return varius information about the kubelet service
func SenseKubeletInfo() (*KubeletInfo, error) {
	ret := KubeletInfo{}

	kubeletProcess, err := LocateKubeletProcess()
	if err != nil {
		return &ret, fmt.Errorf("failed to LocateKubeletProcess: %w", err)
	}

	// Serivce files
	ret.ServiceFiles = makeKubeletServiceFilesInfo(int(kubeletProcess.PID))

	// Kubelet config
	configPath := kubeletConfigDefaultPath
	p, ok := kubeletProcess.GetArg(kubeletConfigArgName)
	if ok {
		configPath = p
	}
	configInfo, err := makeHostFileInfo(configPath, true)
	if err == nil {
		ret.ConfigFile = configInfo
	} else {
		zap.L().Debug("SenseKubeletInfo failed to MakeHostFileInfo for kubelet config",
			zap.String("path", configPath),
			zap.Error(err),
		)
	}

	// Kubelet kubeconfig
	kubeConfigPath := kubeletConfigDefaultPath
	p, ok = kubeletProcess.GetArg(kubeConfigArgName)
	if ok {
		kubeConfigPath = p
	}
	kubeConfigInfo, err := makeHostFileInfo(kubeConfigPath, false)
	if err == nil {
		ret.KubeConfigFile = kubeConfigInfo
	} else {
		zap.L().Debug("SenseKubeletInfo failed to MakeHostFileInfo for kubelet kubeconfig",
			zap.String("path", kubeConfigPath),
			zap.Error(err),
		)
	}

	// Kubelet client ca certificate
	caFilePath, ok := kubeletProcess.GetArg(kubeletClientCAArgName)
	if !ok && configInfo != nil && configInfo.Content != nil {
		zap.L().Error("extracting kubelet client ca certificate from config")
		extracted, err := kubeletExtractCAFileFromConf(configInfo.Content)
		if err == nil {
			caFilePath = extracted
		}
	}
	if caFilePath != "" {
		caInfo, err := makeHostFileInfo(caFilePath, false)
		if err == nil {
			ret.ClientCAFile = caInfo
		} else {
			zap.L().Debug("SenseKubeletInfo failed to MakeHostFileInfo for client ca file",
				zap.String("path", caFilePath),
				zap.Error(err),
			)
		}
	}

	// Cmd line
	ret.CmdLine = kubeletProcess.RawCmd()

	return &ret, nil
}

// kubeletExtractCAFileFromConf extract the client ca file path from kubelet config
func kubeletExtractCAFileFromConf(content []byte) (string, error) {

	confObj := map[string]interface{}{}
	err := yaml.Unmarshal(content, &confObj)
	if err != nil {
		return "", err
	}

	auth, ok := confObj["authentication"].(map[string]interface{})
	if !ok {
		return "", nil
	}

	x509, ok := auth["x509"].(map[string]interface{})
	if !ok {
		return "", nil
	}

	clientCAFile, ok := x509["clientCAFile"].(string)
	if !ok {
		return "", nil
	}

	return clientCAFile, nil
}

// Deprecated: use SenseKubeletInfo for more information.
// Return the content of kubelet config file
func SenseKubeletConfigurations() ([]byte, error) {
	kubeletProcess, err := LocateKubeletProcess()
	if err != nil {
		return nil, fmt.Errorf("failed to LocateKubeletProcess: %w", err)
	}
	kubeletConfFileLocation, ok := kubeletProcess.GetArg(kubeletConfigArgName)
	if !ok || kubeletConfFileLocation == "" {
		return nil, fmt.Errorf("in SenseKubeletConfigurations failed to find kubelet config File location")
	}

	zap.L().Debug("config loaction", zap.String("kubeletConfFileLocation", kubeletConfFileLocation))
	return ReadKubeletConfig(kubeletConfFileLocation)
}
