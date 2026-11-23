package model

// Asset 素材主记录：一个素材可挂多个不同格式文件
type Asset struct {
	ID           int64  `json:"id"`
	AssetName    string `json:"assetName"`
	DisplayName  string `json:"displayName"`
	CategoryID   int64  `json:"categoryId"`
	CategoryName string `json:"categoryName,omitempty"`
	Remark       string `json:"remark"`
	Sort         int    `json:"sort"`
	CreateBy     string `json:"createBy"`
	CreateTime   string `json:"createTime"`
	UpdateTime   string `json:"updateTime"`
	FileCount    int    `json:"fileCount"`
	PrimaryURL   string `json:"primaryUrl"`
	Tags         []Tag  `json:"tags"`
}

type Category struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	OssPrefix  string `json:"ossPrefix"`
	Sort       int    `json:"sort"`
	CreateTime string `json:"createTime"`
	UpdateTime string `json:"updateTime"`
}

type Tag struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	CreateTime string `json:"createTime,omitempty"`
}

// AssetFile 素材的单个格式文件
type AssetFile struct {
	ID         int64  `json:"id"`
	AssetID    int64  `json:"assetId"`
	FileName   string `json:"fileName"`
	Suffix     string `json:"suffix"`
	FileSize   int64  `json:"fileSize"`
	Width      *int   `json:"width"`
	Height     *int   `json:"height"`
	ObjectKey  string `json:"objectKey"`
	URL        string `json:"url"`
	IsPrimary  int    `json:"isPrimary"`
	Remark     string `json:"remark"`
	CreateTime string `json:"createTime"`
}