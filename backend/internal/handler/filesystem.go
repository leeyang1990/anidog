package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type FileSystemHandler struct {
	downloadDir string
}

func NewFileSystemHandler(downloadDir string) *FileSystemHandler {
	return &FileSystemHandler{
		downloadDir: downloadDir,
	}
}

// DirectoryResponse 目录响应
type DirectoryResponse struct {
	Name      string                    `json:"name"`
	Path      string                    `json:"path"`
	IsDir     bool                      `json:"is_dir"`
	Size      int64                     `json:"size"`
	Children  []FileSystemEntryResponse `json:"children,omitempty"`
	ParentPath string                   `json:"parent_path,omitempty"`
}

// FileSystemEntryResponse 文件系统条目响应
type FileSystemEntryResponse struct {
	ID        uint                        `json:"id"`
	Name      string                      `json:"name"`
	Path      string                      `json:"path"`
	IsDir     bool                        `json:"is_dir"`
	Size      int64                       `json:"size"`
	CreatedAt string                      `json:"created_at"`
	Children  []FileSystemEntryResponse  `json:"children,omitempty"`
}

// ListDirectoryRequest 列出目录请求
type ListDirectoryRequest struct {
	Path string `json:"path"`
}

// CreateDirectoryRequest 创建目录请求
type CreateDirectoryRequest struct {
	Path    string `json:"path"`
	DirName string `json:"dir_name" binding:"required"`
}

// DeleteEntryRequest 删除条目请求
type DeleteEntryRequest struct {
	Path string `json:"path" binding:"required"`
	Name string `json:"name" binding:"required"`
}

// ListDirectory 列出目录内容（只列当前层）
func (h *FileSystemHandler) ListDirectory(c *gin.Context) {
	var req ListDirectoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	// 清理路径，防止 "../" 越权
	cleanPath := filepath.Clean(req.Path)
	if cleanPath == "." || cleanPath == "/" {
		cleanPath = ""
	}
	fullPath := filepath.Join(h.downloadDir, cleanPath)

	// 确保路径在 downloadDir 内
	absFull, _ := filepath.Abs(fullPath)
	absRoot, _ := filepath.Abs(h.downloadDir)
	if !strings.HasPrefix(absFull, absRoot) {
		c.JSON(http.StatusForbidden, gin.H{"error": "禁止访问该路径"})
		return
	}

	// 不存在的路径回退到 root（避免浏览器看到 404）
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		cleanPath = ""
		fullPath = h.downloadDir
		if _, err2 := os.Stat(fullPath); os.IsNotExist(err2) {
			// root 也不存在，返回空
			c.JSON(http.StatusOK, gin.H{
				"name":     filepath.Base(h.downloadDir),
				"path":     "",
				"is_dir":   true,
				"children": []interface{}{},
			})
			return
		}
	}

	files, err := os.ReadDir(fullPath)
	if err != nil {
		zap.L().Error("读取目录失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取目录失败"})
		return
	}

	var entries []FileSystemEntryResponse
	for _, f := range files {
		info, err := f.Info()
		if err != nil {
			continue
		}
		entryRel := filepath.Join(cleanPath, f.Name())
		if cleanPath == "" {
			entryRel = f.Name()
		}
		entries = append(entries, FileSystemEntryResponse{
			Name:  f.Name(),
			Path:  entryRel,
			IsDir: info.IsDir(),
			Size:  info.Size(),
		})
	}

	parentPath := filepath.Dir(cleanPath)
	if parentPath == "." || parentPath == "/" {
		parentPath = ""
	}

	c.JSON(http.StatusOK, DirectoryResponse{
		Name:       filepath.Base(cleanPath),
		Path:       cleanPath,
		IsDir:      true,
		Children:   entries,
		ParentPath: parentPath,
	})
}

// CreateDirectory 创建目录
func (h *FileSystemHandler) CreateDirectory(c *gin.Context) {
	var req CreateDirectoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	fullPath := filepath.Join(h.downloadDir, req.Path, req.DirName)
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		zap.L().Error("创建目录失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建目录失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "目录创建成功"})
}

// DeleteEntry 删除条目（文件或目录）
func (h *FileSystemHandler) DeleteEntry(c *gin.Context) {
	var req DeleteEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的请求参数"})
		return
	}

	fullPath := filepath.Join(h.downloadDir, req.Path, req.Name)
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件或目录不存在"})
		return
	}

	if err := os.RemoveAll(fullPath); err != nil {
		zap.L().Error("删除失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// listDirectoryContents 递归列出目录内容
func (h *FileSystemHandler) listDirectoryContents(dirPath string, relativePath string) ([]FileSystemEntryResponse, error) {
	var entries []FileSystemEntryResponse

	files, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		fileName := file.Name()
		fullPath := filepath.Join(dirPath, fileName)
		info, err := file.Info()
		if err != nil {
			continue
		}

		entryRelativePath := filepath.Join(relativePath, fileName)
		if relativePath == "." || relativePath == "" {
			entryRelativePath = fileName
		}

		entry := FileSystemEntryResponse{
			Name:  fileName,
			Path:  entryRelativePath,
			IsDir: info.IsDir(),
			Size:  info.Size(),
		}

		if !info.IsDir() {
			entries = append(entries, entry)
		} else {
			// 对于目录，我们单独处理
			subEntries, err := h.listDirectoryContents(fullPath, entryRelativePath)
			if err == nil {
				entry.Size = h.calculateDirectorySize(subEntries)
				entries = append(entries, entry)
			}
		}
	}

	return entries, nil
}

// calculateDirectorySize 计算目录总大小
func (h *FileSystemHandler) calculateDirectorySize(entries []FileSystemEntryResponse) int64 {
	var totalSize int64
	for _, entry := range entries {
		if entry.IsDir {
			if len(entry.Children) > 0 {
				totalSize += entry.Size
			}
		} else {
			totalSize += entry.Size
		}
	}
	return totalSize
}

// GetRootDirectories 获取根目录列表
func (h *FileSystemHandler) GetRootDirectories(c *gin.Context) {
	entries, err := os.ReadDir(h.downloadDir)
	if err != nil {
		zap.L().Error("读取下载目录失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取下载目录失败"})
		return
	}

	var rootEntries []FileSystemEntryResponse
	for _, entry := range entries {
		entryName := entry.Name()
		fullPath := filepath.Join(h.downloadDir, entryName)
		info, err := entry.Info()
		if err != nil {
			continue
		}

		rootEntry := FileSystemEntryResponse{
			Name:  entryName,
			Path:  entryName,
			IsDir: info.IsDir(),
			Size:  info.Size(),
		}

		if info.IsDir() {
			// 获取子目录信息
			subEntries, err := h.listDirectoryContents(fullPath, entryName)
			if err == nil {
				rootEntry.Size = h.calculateDirectorySize(subEntries)
			}
		}

		rootEntries = append(rootEntries, rootEntry)
	}

	c.JSON(http.StatusOK, rootEntries)
}

// RegisterRoutes 注册路由
func (h *FileSystemHandler) RegisterRoutes(r *gin.RouterGroup) {
	fs := r.Group("/filesystem")
	fs.POST("/list", h.ListDirectory)
	fs.POST("/create", h.CreateDirectory)
	fs.POST("/delete", h.DeleteEntry)
	fs.GET("/root", h.GetRootDirectories)
}
