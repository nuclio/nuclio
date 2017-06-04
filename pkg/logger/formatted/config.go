package formatted

type OutputConfig struct {
	Level string
}

// base file output config
type FileOutputConfig struct {
	OutputConfig
	FullPath    string
	MaxNumFiles int
}

type FileRotatedOutputConfig struct {
	FileOutputConfig
	MaxFileSizeMB int
}

type FileTimedOutputConfig struct {
	FileOutputConfig
	Period int
}

type StdoutOutputConfig struct {
	OutputConfig
	Colors string // off / on (if tty) / always
}
