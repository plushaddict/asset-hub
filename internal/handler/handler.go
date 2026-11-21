package handler

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"

	"assethub/internal/config"
	"assethub/internal/model"
	"assethub/internal/service"
)

type Handler struct {
	assetSvc    *service.AssetService
	settingsSvc *service.SettingsService
	catSvc      *service.CategoryService
	tagSvc      *service.TagService
	cfg         *config.Config
	pages       fs.FS
}

func New(assetSvc *service.AssetService, settingsSvc *service.SettingsService, catSvc *service.CategoryService, tagSvc *service.TagService, cfg *config.Config, pages fs.FS) *Handler {
	return &Handler{
		assetSvc:    assetSvc,
		settingsSvc: settingsSvc,
		catSvc:      catSvc,
		tagSvc:      tagSvc,
		cfg:         cfg,
		pages:       pages,
	}
}

// ---------- 通用 ----------

func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": data})
}

func fail(c *gin.Context, msg string) {
	c.JSON(http.StatusOK, gin.H{"code": -1, "msg": msg})
}

func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func parseInt64(s string, def int64) int64 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return def
	}
	return v
}

// parseIDList 把逗号分隔的 id 拆成正整数列表。空串返回 nil。
func parseIDList(s string) []int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]int64, 0, len(parts))
	for _, p := range parts {
		v := parseInt64(strings.TrimSpace(p), 0)
		if v > 0 {
			out = append(out, v)
		}
	}
	return out
}

func (h *Handler) render(c *gin.Context, name string) {
	b, err := fs.ReadFile(h.pages, name)
	if err != nil {
		c.String(http.StatusInternalServerError, "页面不存在: %s", name)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", b)
}

// AuthPage 页面鉴权：未登录一律跳转登录页
func (h *Handler) AuthPage() gin.HandlerFunc {
	return func(c *gin.Context) {
		sess := sessions.Default(c)
		if v, _ := sess.Get("login").(bool); !v {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}

// AuthAPI 接口鉴权：未登录返回 401 JSON
func (h *Handler) AuthAPI() gin.HandlerFunc {
	return func(c *gin.Context) {
		sess := sessions.Default(c)
		if v, _ := sess.Get("login").(bool); !v {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录或会话已过期"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// ---------- 页面 ----------

func (h *Handler) LoginPage(c *gin.Context)    { h.render(c, "login.html") }
func (h *Handler) IndexPage(c *gin.Context)    { h.render(c, "index.html") }
func (h *Handler) DetailPage(c *gin.Context)   { h.render(c, "detail.html") }
func (h *Handler) SettingsPage(c *gin.Context) { h.render(c, "settings.html") }

// ---------- 登录 ----------

func (h *Handler) Login(c *gin.Context) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, "参数错误")
		return
	}
	if body.Username != h.cfg.Server.Username || body.Password != h.cfg.Server.Password {
		fail(c, "账号或密码错误")
		return
	}
	sess := sessions.Default(c)
	sess.Set("login", true)
	sess.Set("username", body.Username)
	sess.Options(sessions.Options{
		Path:     "/",
		MaxAge:   h.cfg.Server.SessionMaxAge,
		HttpOnly: true,
	})
	_ = sess.Save()
	ok(c, nil)
}

func (h *Handler) Logout(c *gin.Context) {
	sess := sessions.Default(c)
	sess.Clear()
	_ = sess.Save()
	ok(c, nil)
}

// ---------- 设置 / 引导 ----------

func (h *Handler) GetSettings(c *gin.Context) {
	ok(c, h.settingsSvc.View())
}

func (h *Handler) PutSettings(c *gin.Context) {
	var req service.SettingsUpdateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, "参数错误")
		return
	}
	v, err := h.settingsSvc.Update(req)
	if err != nil {
		fail(c, err.Error())
		return
	}
	ok(c, v)
}

func (h *Handler) SetupStatus(c *gin.Context) {
	ok(c, h.settingsSvc.SetupStatus())
}

// ---------- 分类 ----------

type categoryBody struct {
	Name      string `json:"name"`
	OssPrefix string `json:"ossPrefix"`
	Sort      int    `json:"sort"`
}

func (h *Handler) ListCategories(c *gin.Context) {
	list, err := h.catSvc.List()
	if err != nil {
		fail(c, err.Error())
		return
	}
	if list == nil {
		list = []model.Category{}
	}
	ok(c, list)
}

func (h *Handler) CreateCategory(c *gin.Context) {
	var body categoryBody
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, "参数错误")
		return
	}
	cat, err := h.catSvc.Create(body.Name, body.OssPrefix, body.Sort)
	if err != nil {
		fail(c, err.Error())
		return
	}
	ok(c, cat)
}

func (h *Handler) UpdateCategory(c *gin.Context) {
	id := parseInt64(c.Param("id"), 0)
	var body categoryBody
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, "参数错误")
		return
	}
	cat, err := h.catSvc.Update(id, body.Name, body.OssPrefix, body.Sort)
	if err != nil {
		fail(c, err.Error())
		return
	}
	ok(c, cat)
}

func (h *Handler) DeleteCategory(c *gin.Context) {
	id := parseInt64(c.Param("id"), 0)
	if err := h.catSvc.Delete(id); err != nil {
		fail(c, err.Error())
		return
	}
	ok(c, nil)
}

// ---------- 标签 ----------

type tagBody struct {
	Name string `json:"name"`
}

func (h *Handler) ListTags(c *gin.Context) {
	list, err := h.tagSvc.List()
	if err != nil {
		fail(c, err.Error())
		return
	}
	if list == nil {
		list = []model.Tag{}
	}
	ok(c, list)
}

func (h *Handler) CreateTag(c *gin.Context) {
	var body tagBody
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, "参数错误")
		return
	}
	t, err := h.tagSvc.Create(body.Name)
	if err != nil {
		fail(c, err.Error())
		return
	}
	ok(c, t)
}

func (h *Handler) UpdateTag(c *gin.Context) {
	id := parseInt64(c.Param("id"), 0)
	var body tagBody
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, "参数错误")
		return
	}
	t, err := h.tagSvc.Update(id, body.Name)
	if err != nil {
		fail(c, err.Error())
		return
	}
	ok(c, t)
}

func (h *Handler) DeleteTag(c *gin.Context) {
	id := parseInt64(c.Param("id"), 0)
	if err := h.tagSvc.Delete(id); err != nil {
		fail(c, err.Error())
		return
	}
	ok(c, nil)
}

// ---------- 素材 ----------

func (h *Handler) CreateAsset(c *gin.Context) {
	var body struct {
		AssetName  string   `json:"assetName"`
		CategoryID int64    `json:"categoryId"`
		TagIDs     []int64  `json:"tagIds"`
		TagNames   []string `json:"tagNames"`
		Remark     string   `json:"remark"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, "参数错误")
		return
	}
	sess := sessions.Default(c)
	createBy, _ := sess.Get("username").(string)
	a, err := h.assetSvc.CreateAsset(body.AssetName, body.CategoryID, body.TagIDs, body.TagNames, body.Remark, createBy)
	if err != nil {
		fail(c, err.Error())
		return
	}
	ok(c, a)
}

func (h *Handler) AssetList(c *gin.Context) {
	keyword := c.Query("keyword")
	categoryID := parseInt64(c.Query("categoryId"), 0)
	tagIDs := parseIDList(c.Query("tagIds"))
	page := parseInt(c.Query("page"), 1)
	size := parseInt(c.Query("size"), 12)

	list, total, err := h.assetSvc.PageList(keyword, categoryID, tagIDs, page, size)
	if err != nil {
		fail(c, err.Error())
		return
	}
	if list == nil {
		list = []*model.Asset{}
	}
	ok(c, gin.H{"list": list, "total": total, "page": page, "size": size})
}

func (h *Handler) AssetDetail(c *gin.Context) {
	id := parseInt64(c.Param("id"), 0)
	a, err := h.assetSvc.GetAsset(id)
	if err != nil {
		fail(c, "素材不存在")
		return
	}
	files, err := h.assetSvc.ListFiles(a.ID)
	if err != nil {
		fail(c, err.Error())
		return
	}
	if files == nil {
		files = []*model.AssetFile{}
	}
	ok(c, gin.H{"asset": a, "files": files})
}

func (h *Handler) UpdateAsset(c *gin.Context) {
	id := parseInt64(c.Param("id"), 0)
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil || len(raw) == 0 {
		fail(c, "参数错误")
		return
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		fail(c, "参数错误")
		return
	}
	var body struct {
		AssetName  string   `json:"assetName"`
		CategoryID int64    `json:"categoryId"`
		TagIDs     []int64  `json:"tagIds"`
		TagNames   []string `json:"tagNames"`
		Remark     string   `json:"remark"`
		Sort       int      `json:"sort"`
	}
	if err = json.Unmarshal(raw, &body); err != nil {
		fail(c, "参数错误")
		return
	}
	_, hasTagIDs := fields["tagIds"]
	_, hasTagNames := fields["tagNames"]
	tagsProvided := hasTagIDs || hasTagNames
	if err = h.assetSvc.UpdateAsset(id, body.AssetName, body.CategoryID, tagsProvided, body.TagIDs, body.TagNames, body.Remark, body.Sort); err != nil {
		fail(c, err.Error())
		return
	}
	ok(c, nil)
}

func (h *Handler) DeleteAsset(c *gin.Context) {
	id := parseInt64(c.Param("id"), 0)
	if err := h.assetSvc.DeleteAsset(id); err != nil {
		fail(c, err.Error())
		return
	}
	ok(c, nil)
}

// ---------- 文件 ----------

func (h *Handler) UploadFile(c *gin.Context) {
	id := parseInt64(c.Param("id"), 0)
	header, err := c.FormFile("file")
	if err != nil {
		fail(c, "请选择要上传的文件")
		return
	}
	f, err := h.assetSvc.UploadFile(id, header)
	if err != nil {
		fail(c, err.Error())
		return
	}
	ok(c, f)
}

func (h *Handler) SetPrimary(c *gin.Context) {
	assetID := parseInt64(c.Param("id"), 0)
	fileID := parseInt64(c.Param("fid"), 0)
	if err := h.assetSvc.SetPrimary(assetID, fileID); err != nil {
		fail(c, err.Error())
		return
	}
	ok(c, nil)
}

func (h *Handler) DeleteFile(c *gin.Context) {
	assetID := parseInt64(c.Param("id"), 0)
	fileID := parseInt64(c.Param("fid"), 0)
	if err := h.assetSvc.DeleteFile(assetID, fileID); err != nil {
		fail(c, err.Error())
		return
	}
	ok(c, nil)
}

// ---------- 业务查询（给 ComiGo 等业务调用） ----------

func (h *Handler) BusinessAsset(c *gin.Context) {
	if tokenExpected := h.settingsSvc.BusinessToken(); tokenExpected != "" {
		token := c.Query("token")
		if token == "" {
			token = c.GetHeader("X-Asset-Token")
		}
		if token != tokenExpected {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "token 无效"})
			return
		}
	}

	var a *model.Asset
	var err error
	if name := c.Query("assetName"); name != "" {
		a, err = h.assetSvc.GetAssetByName(name)
	} else if idStr := c.Query("assetId"); idStr != "" {
		a, err = h.assetSvc.GetAsset(parseInt64(idStr, 0))
	} else {
		fail(c, "assetName 或 assetId 至少传一个")
		return
	}
	if err != nil {
		fail(c, "素材不存在")
		return
	}

	files, err := h.assetSvc.ListFiles(a.ID)
	if err != nil {
		fail(c, err.Error())
		return
	}
	if files == nil {
		files = []*model.AssetFile{}
	}
	if a.Tags == nil {
		a.Tags = []model.Tag{}
	}
	ok(c, gin.H{
		"assetId":      a.ID,
		"assetName":    a.AssetName,
		"categoryId":   a.CategoryID,
		"categoryName": a.CategoryName,
		"tags":         a.Tags,
		"remark":       a.Remark,
		"fileList":     files,
	})
}
