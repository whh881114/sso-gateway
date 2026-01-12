package models

// ServerConfig 服务器配置
type ServerConfig struct {
	Port       int    `yaml:"port"`
	SessionKey string `yaml:"session_key"`
}

// RouteConfig 路由配置
type RouteConfig struct {
	Name   string `yaml:"name"`
	Path   string `yaml:"path"`
	Target string `yaml:"target"`
}

// CASConfig CAS 认证配置
type CASConfig struct {
	BaseURL      string `yaml:"base_url"`
	LoginPath    string `yaml:"login_path"`    // 可选，默认为 "/login"
	ValidatePath string `yaml:"validate_path"` // 可选，默认为 "/p3/serviceValidate"
	UseJSON      bool   `yaml:"use_json"`      // 是否使用JSON格式（添加format=json参数）
}

// Config 主配置结构
type Config struct {
	Server ServerConfig `yaml:"server"`
	CAS    CASConfig    `yaml:"cas"`
	Route  RouteConfig  `yaml:"route"` // 路由配置（单个路由）
}
