package ceserver

import "fmt"

type ServerInfo struct {
	Endpoint        string `json:"endpoint"`
	ProtocolVersion int32  `json:"protocol_version"`
	VersionName     string `json:"version_name"`
	ABI             string `json:"abi"`
}

type Process struct {
	PID  int32  `json:"pid"`
	Name string `json:"name"`
}

type Module struct {
	BaseAddress uint64 `json:"base_address"`
	Size        uint32 `json:"size"`
	FileOffset  uint32 `json:"file_offset"`
	Part        int32  `json:"part"`
	Name        string `json:"name"`
}

type Thread struct {
	TID int32 `json:"tid"`
}

type Region struct {
	BaseAddress uint64     `json:"base_address"`
	Size        uint64     `json:"size"`
	Protection  Protection `json:"protection_code"`
	Permissions string     `json:"permissions"`
	Type        MemoryType `json:"type_code"`
	TypeName    string     `json:"type"`
}

type RegionDetail struct {
	Region
	MapsLine string `json:"maps_line"`
}

type Symbol struct {
	Address uint64 `json:"address"`
	Size    int32  `json:"size"`
	Type    int32  `json:"type"`
	Name    string `json:"name"`
}

type SymbolList struct {
	Path        string   `json:"path"`
	FileOffset  uint32   `json:"file_offset"`
	Executable  bool     `json:"executable"`
	Symbols     []Symbol `json:"symbols"`
	SymbolCount int      `json:"symbol_count"`
}

type ProcessInfo struct {
	PID          int32  `json:"pid"`
	Architecture string `json:"architecture"`
	ModuleCount  int    `json:"module_count"`
	ThreadCount  int    `json:"thread_count"`
}

type ServerPathInfo struct {
	ExecutablePath string `json:"executable_path"`
	CurrentPath    string `json:"current_path"`
	Android        bool   `json:"android"`
}

type ServerOption struct {
	Name             string `json:"name"`
	Parent           string `json:"parent,omitempty"`
	Description      string `json:"description"`
	AcceptableValues string `json:"acceptable_values,omitempty"`
	CurrentValue     string `json:"current_value"`
	TypeCode         int32  `json:"type_code"`
	Type             string `json:"type"`
}

type RemoteFile struct {
	Name     string `json:"name"`
	TypeCode uint8  `json:"type_code"`
	Type     string `json:"type"`
}

type ProtectionChange struct {
	Address        uint64     `json:"address"`
	Size           uint32     `json:"size"`
	OldProtection  Protection `json:"old_protection_code"`
	OldPermissions string     `json:"old_permissions"`
	NewProtection  Protection `json:"new_protection_code"`
	NewPermissions string     `json:"new_permissions"`
}

type ProtocolError struct {
	Operation string
	Message   string
}

func (protocolError *ProtocolError) Error() string {
	return fmt.Sprintf("ceserver %s: %s", protocolError.Operation, protocolError.Message)
}
