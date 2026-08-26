package models

// Media 表示列表中的一个听力材料条目
type Media struct {
	MediaID     string
	Title       string
	FileTime    string
	Status      string
	DetailToken string
}

// Sentence 表示一个带时间戳的句子
type Sentence struct {
	Index       int
	ID          string
	Start       float64
	End         float64
	Text        string
	Translation string
}

// Detail 表示一个听力材料的完整详情
type Detail struct {
	MediaID   string
	Title     string
	Sentences []Sentence
	AudioURL  string
}

// AppConfig 应用配置
type AppConfig struct {
	EudicSession  string `toml:"eudic_session"`
	AspNetSession string `toml:"aspnet_session"`
	ChannelID     string `toml:"channel_id"`
	CacheDir      string `toml:"cache_dir"`
	DefaultSpeed  float64 `toml:"default_speed"`
	DefaultVolume int    `toml:"default_volume"`

	// ConfigPath 当前配置文件路径（不写入 toml，仅运行时记录便于 Save）
	ConfigPath string `toml:"-"`
}
