package upload

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// SaveAvatarFromForm 保存头像文件并返回可访问 URL（当前实现与历史行为一致）。
func SaveAvatarFromForm(c *gin.Context) (remoteURL string, httpStatus int, err error) {
	file, err := c.FormFile("file")
	if err != nil {
		return "", http.StatusBadRequest, fmt.Errorf("缺少文件字段 file: %w", err)
	}

	saveDir := "./photos"
	if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
		return "", http.StatusInternalServerError, fmt.Errorf("创建目录失败: %w", err)
	}

	ext := filepath.Ext(file.Filename)
	newFileName := fmt.Sprintf("%d_%d%s", time.Now().UnixNano(), os.Getpid(), ext)
	savePath := filepath.Join(saveDir, newFileName)

	if err := c.SaveUploadedFile(file, savePath); err != nil {
		return "", http.StatusInternalServerError, fmt.Errorf("保存文件失败: %w", err)
	}

	return fmt.Sprintf("http://127.0.0.1:8088/photos/%s", newFileName), http.StatusOK, nil
}
