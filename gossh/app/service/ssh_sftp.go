package service

import (
	"errors"
	"fmt"
	"gossh/gin"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

const sftpEditMaxBytes = 2 * 1024 * 1024

func getSshConn(sessionId string) (*SshConn, error) {
	cli, ok := OnlineClients.Load(sessionId)
	if !ok {
		slog.Error("加载ssh连接错误")
		return nil, errors.New("加载ssh连接错误")
	}
	conn, ok := cli.(*SshConn)
	if conn != nil && !ok {
		slog.Error("断言ssh连接错误")
		return nil, errors.New("断言ssh连接错误")
	}
	return conn, nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func execSftpShellCommand(conn *SshConn, cmd string) (string, error) {
	session, err := conn.sshClient.NewSession()
	if err != nil {
		return "", err
	}
	defer func() {
		_ = session.Close()
	}()

	out, err := session.CombinedOutput(cmd)
	return string(out), err
}

// SftpList GET sftp 获取指定目录下文件信息
func SftpList(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			c.JSON(200, gin.H{"code": 4, "msg": "读取目录错误"})
			return
		}
	}()
	dirPath := c.PostForm("path")

	conn, err := getSshConn(c.PostForm("session_id"))
	if err != nil {
		slog.Error(err.Error())
		c.JSON(200, gin.H{"code": 2, "msg": err.Error()})
		return
	}

	files, err := conn.sftpClient.ReadDir(dirPath)
	if err != nil {
		slog.Error("sftp客户端ReadDir错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 3, "msg": "sftp客户端读取目录错误"})
		return
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime().After(files[j].ModTime())
	})

	fileCount := 0
	dirCount := 0
	var fileList []any
	for _, file := range files {
		fileInfo := map[string]any{}
		fileInfo["path"] = path.Join(dirPath, file.Name())
		fileInfo["name"] = file.Name()
		fileInfo["mode"] = file.Mode().String()
		fileInfo["size"] = file.Size()
		fileInfo["mod_time"] = file.ModTime().Format("2006-01-02 15:04:05")
		if file.IsDir() {
			fileInfo["type"] = "d"
			dirCount += 1
		} else {
			fileInfo["type"] = "f"
			fileCount += 1
		}
		fileList = append(fileList, fileInfo)
	}

	// 内部方法,处理路径信息
	pathHandler := func(dirPath string) (paths []map[string]string) {
		tmp := strings.Split(dirPath, "/")

		var dirs []string
		if strings.HasPrefix(dirPath, "/") {
			dirs = append(dirs, "/")
		}

		for _, item := range tmp {
			name := strings.TrimSpace(item)
			if len(name) > 0 {
				dirs = append(dirs, name)
			}
		}

		for i, item := range dirs {
			fullPath := path.Join(dirs[:i+1]...)
			pathInfo := map[string]string{}
			pathInfo["name"] = item
			pathInfo["dir"] = fullPath
			paths = append(paths, pathInfo)
		}
		return paths
	}

	data := map[string]any{
		"files":       fileList,
		"file_count":  fileCount,
		"dir_count":   dirCount,
		"paths":       pathHandler(dirPath),
		"current_dir": dirPath,
	}

	c.JSON(200, gin.H{"code": 0, "data": data, "msg": "ok"})
}

// SftpDownLoad POST sftp 下载文件
func SftpDownLoad(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			c.JSON(200, gin.H{"code": 1, "msg": "下载错误"})
			return
		}
	}()
	fullPath, err := url.QueryUnescape(c.Query("path"))
	if err != nil {
		slog.Error("获取文件路径参数错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 2, "msg": "获取文件路径参数错误"})
		return
	}
	conn, err := getSshConn(c.Query("session_id"))
	if err != nil {
		slog.Error(err.Error())
		c.JSON(200, gin.H{"code": 3, "msg": err.Error()})
		return
	}

	file, err := conn.sftpClient.Open(fullPath)
	if err != nil {
		slog.Error("sftpClient.Openc错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 4, "msg": "sftp打开文件错误"})
		return
	}
	defer func() {
		_ = file.Close()
	}()

	stat, err := file.Stat()
	if err != nil {
		slog.Error("file.Stat()错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 4, "msg": "读取文件信息错误"})
		return
	}

	c.Writer.WriteHeader(http.StatusOK)
	c.Header("Content-Disposition", "attachment; filename="+stat.Name())
	c.Header("Content-Type", "application/octet-stream")
	//c.Header("Content-Type", "application/x-download")
	c.Header("Content-Length", fmt.Sprintf("%d", stat.Size()))
	_, err = file.WriteTo(c.Writer)
	if err != nil {
		slog.Error("file.WriteTo错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 5, "msg": "下载文件错误"})
		return
	}
	c.Writer.Flush()
}

// SftpUpload PUT sftp 上传文件
func SftpUpload(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			c.JSON(200, gin.H{"code": 4, "msg": "上传错误"})
			return
		}
	}()

	dstPath := c.PostForm("path")
	//获取上传的文件组
	form, err := c.MultipartForm()
	if err != nil {
		slog.Error("获取form数据错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 1, "msg": "获取form数据错误"})
		return
	}
	defer func() {
		_ = form.RemoveAll()
	}()
	files := form.File["files"]
	// files := c.Request.MultipartForm.File["file"]

	conn, err := getSshConn(c.PostForm("session_id"))
	if err != nil {
		slog.Error(err.Error())
		c.JSON(200, gin.H{"code": 2, "msg": err.Error()})
		return
	}

	var ret []string
	for _, file := range files {
		srcFile, err := file.Open()
		if err != nil {
			continue
		}
		fileName := file.Filename
		dstFile, err := conn.sftpClient.Create(path.Join(dstPath, fileName))
		if err != nil {
			_ = srcFile.Close()
			continue
		}
		_, err = io.Copy(dstFile, srcFile)
		srcErr := srcFile.Close()
		dstErr := dstFile.Close()
		if err != nil || srcErr != nil || dstErr != nil {
			continue
		}
		ret = append(ret, fileName)
	}
	msg := strconv.Itoa(len(ret)) + " 个文件上传成功"
	c.JSON(200, gin.H{"code": 0, "msg": msg, "data": ret})
}

// SftpDelete DELETE sftp 删除文件或目录
func SftpDelete(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			c.JSON(200, gin.H{"code": 4, "msg": "删除错误"})
			return
		}
	}()

	type Body struct {
		SessionId string `form:"session_id" binding:"required,min=1,max=128" json:"session_id"`
		Path      string `form:"path" binding:"required,min=1,max=1024" json:"path"`
	}

	var body Body
	if err := c.ShouldBind(&body); err != nil {
		slog.Error("绑定数据错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 1, "msg": "输入数据不合法"})
		return
	}
	conn, err := getSshConn(body.SessionId)
	if err != nil {
		slog.Error(err.Error())
		c.JSON(200, gin.H{"code": 1, "msg": err.Error()})
		return
	}

	err = conn.sftpClient.RemoveAll(body.Path)
	if err != nil {
		slog.Error("sftpClient.Remove错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 2, "msg": "删除文件错误"})
		return
	}
	c.JSON(200, gin.H{"code": 0, "msg": "删除成功"})
}

// SftpCreateDir sftp 创建目录
func SftpCreateDir(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			c.JSON(200, gin.H{"code": 4, "msg": "创建目录错误"})
			return
		}
	}()

	type Body struct {
		SessionId string `form:"session_id" binding:"required,min=1,max=128" json:"session_id"`
		Path      string `form:"path" binding:"required,min=1,max=1024" json:"path"`
	}

	var body Body
	if err := c.ShouldBind(&body); err != nil {
		slog.Error("绑定数据错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 1, "msg": "输入数据不合法"})
		return
	}
	conn, err := getSshConn(body.SessionId)
	if err != nil {
		slog.Error(err.Error())
		c.JSON(200, gin.H{"code": 1, "msg": err.Error()})
		return
	}

	err = conn.sftpClient.MkdirAll(body.Path)
	if err != nil {
		slog.Error("sftpClient.MkdirAll错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 2, "msg": "创建目录错误"})
		return
	}
	c.JSON(200, gin.H{"code": 0, "msg": "创建目录成功"})
}

// SftpCreateFile sftp 创建空文件
func SftpCreateFile(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			c.JSON(200, gin.H{"code": 4, "msg": "创建文件错误"})
			return
		}
	}()

	type Body struct {
		SessionId string `form:"session_id" binding:"required,min=1,max=128" json:"session_id"`
		Path      string `form:"path" binding:"required,min=1,max=1024" json:"path"`
	}

	var body Body
	if err := c.ShouldBind(&body); err != nil {
		slog.Error("绑定数据错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 1, "msg": "输入数据不合法"})
		return
	}
	conn, err := getSshConn(body.SessionId)
	if err != nil {
		slog.Error(err.Error())
		c.JSON(200, gin.H{"code": 1, "msg": err.Error()})
		return
	}

	file, err := conn.sftpClient.OpenFile(body.Path, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		slog.Error("sftpClient.OpenFile创建文件错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 2, "msg": "创建文件错误, 文件可能已存在"})
		return
	}
	defer func() {
		_ = file.Close()
	}()

	c.JSON(200, gin.H{"code": 0, "msg": "创建文件成功"})
}

// SftpRename sftp 重命名文件或目录
func SftpRename(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			c.JSON(200, gin.H{"code": 4, "msg": "重命名错误"})
			return
		}
	}()

	type Body struct {
		SessionId string `form:"session_id" binding:"required,min=1,max=128" json:"session_id"`
		OldPath   string `form:"old_path" binding:"required,min=1,max=1024" json:"old_path"`
		NewPath   string `form:"new_path" binding:"required,min=1,max=1024" json:"new_path"`
	}

	var body Body
	if err := c.ShouldBind(&body); err != nil {
		slog.Error("绑定数据错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 1, "msg": "输入数据不合法"})
		return
	}
	conn, err := getSshConn(body.SessionId)
	if err != nil {
		slog.Error(err.Error())
		c.JSON(200, gin.H{"code": 1, "msg": err.Error()})
		return
	}

	if err := conn.sftpClient.Rename(body.OldPath, body.NewPath); err != nil {
		slog.Error("sftpClient.Rename错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 2, "msg": "重命名错误"})
		return
	}
	c.JSON(200, gin.H{"code": 0, "msg": "重命名成功"})
}

// SftpChmod sftp 修改文件或目录权限
func SftpChmod(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			c.JSON(200, gin.H{"code": 4, "msg": "设置权限错误"})
			return
		}
	}()

	type Body struct {
		SessionId string `form:"session_id" binding:"required,min=1,max=128" json:"session_id"`
		Path      string `form:"path" binding:"required,min=1,max=1024" json:"path"`
		Mode      string `form:"mode" binding:"required,min=3,max=4" json:"mode"`
	}

	var body Body
	if err := c.ShouldBind(&body); err != nil {
		slog.Error("绑定数据错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 1, "msg": "输入数据不合法"})
		return
	}

	modeText := strings.TrimSpace(body.Mode)
	modeValue, err := strconv.ParseUint(modeText, 8, 32)
	if err != nil || modeValue > 07777 {
		c.JSON(200, gin.H{"code": 2, "msg": "权限格式错误, 请输入 644 或 0755"})
		return
	}

	conn, err := getSshConn(body.SessionId)
	if err != nil {
		slog.Error(err.Error())
		c.JSON(200, gin.H{"code": 3, "msg": err.Error()})
		return
	}

	if err := conn.sftpClient.Chmod(body.Path, os.FileMode(modeValue)); err != nil {
		slog.Error("sftpClient.Chmod错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 4, "msg": "设置权限错误"})
		return
	}
	c.JSON(200, gin.H{"code": 0, "msg": "设置权限成功"})
}

// SftpReadFile sftp 读取文本文件内容
func SftpReadFile(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			c.JSON(200, gin.H{"code": 4, "msg": "读取文件错误"})
			return
		}
	}()

	type Body struct {
		SessionId string `form:"session_id" binding:"required,min=1,max=128" json:"session_id"`
		Path      string `form:"path" binding:"required,min=1,max=1024" json:"path"`
	}

	var body Body
	if err := c.ShouldBind(&body); err != nil {
		slog.Error("绑定数据错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 1, "msg": "输入数据不合法"})
		return
	}
	conn, err := getSshConn(body.SessionId)
	if err != nil {
		slog.Error(err.Error())
		c.JSON(200, gin.H{"code": 2, "msg": err.Error()})
		return
	}

	file, err := conn.sftpClient.Open(body.Path)
	if err != nil {
		slog.Error("sftpClient.Open错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 3, "msg": "打开文件错误"})
		return
	}
	defer func() {
		_ = file.Close()
	}()

	content, err := io.ReadAll(io.LimitReader(file, sftpEditMaxBytes+1))
	if err != nil {
		slog.Error("读取文件内容错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 4, "msg": "读取文件错误"})
		return
	}
	if len(content) > sftpEditMaxBytes {
		c.JSON(200, gin.H{"code": 5, "msg": "文件超过 2MB, 请下载后编辑"})
		return
	}

	c.JSON(200, gin.H{"code": 0, "msg": "ok", "data": gin.H{"path": body.Path, "content": string(content)}})
}

// SftpSaveFile sftp 保存文本文件内容
func SftpSaveFile(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			c.JSON(200, gin.H{"code": 4, "msg": "保存文件错误"})
			return
		}
	}()

	type Body struct {
		SessionId string `form:"session_id" binding:"required,min=1,max=128" json:"session_id"`
		Path      string `form:"path" binding:"required,min=1,max=1024" json:"path"`
		Content   string `form:"content" json:"content"`
	}

	var body Body
	if err := c.ShouldBind(&body); err != nil {
		slog.Error("绑定数据错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 1, "msg": "输入数据不合法"})
		return
	}
	if len([]byte(body.Content)) > sftpEditMaxBytes {
		c.JSON(200, gin.H{"code": 2, "msg": "文件超过 2MB, 请下载后编辑"})
		return
	}

	conn, err := getSshConn(body.SessionId)
	if err != nil {
		slog.Error(err.Error())
		c.JSON(200, gin.H{"code": 3, "msg": err.Error()})
		return
	}

	file, err := conn.sftpClient.OpenFile(body.Path, os.O_WRONLY|os.O_TRUNC)
	if err != nil {
		slog.Error("sftpClient.OpenFile错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 4, "msg": "打开文件错误"})
		return
	}
	defer func() {
		_ = file.Close()
	}()

	if _, err := io.Copy(file, strings.NewReader(body.Content)); err != nil {
		slog.Error("写入文件内容错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 5, "msg": "保存文件错误"})
		return
	}

	c.JSON(200, gin.H{"code": 0, "msg": "保存成功"})
}

// SftpCompressDir sftp 压缩目录为 tar.gz
func SftpCompressDir(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			c.JSON(200, gin.H{"code": 4, "msg": "压缩目录错误"})
			return
		}
	}()

	type Body struct {
		SessionId string `form:"session_id" binding:"required,min=1,max=128" json:"session_id"`
		Path      string `form:"path" binding:"required,min=1,max=1024" json:"path"`
	}

	var body Body
	if err := c.ShouldBind(&body); err != nil {
		slog.Error("绑定数据错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 1, "msg": "输入数据不合法"})
		return
	}
	conn, err := getSshConn(body.SessionId)
	if err != nil {
		slog.Error(err.Error())
		c.JSON(200, gin.H{"code": 2, "msg": err.Error()})
		return
	}

	dirPath := path.Clean(body.Path)
	parentDir := path.Dir(dirPath)
	baseName := path.Base(dirPath)
	if baseName == "." || baseName == "/" || baseName == "" {
		c.JSON(200, gin.H{"code": 3, "msg": "目录路径不合法"})
		return
	}

	archiveName := baseName + ".tar.gz"
	archivePath := path.Join(parentDir, archiveName)
	cmd := fmt.Sprintf("cd %s && tar -czf %s %s", shellQuote(parentDir), shellQuote(archiveName), shellQuote(baseName))
	out, err := execSftpShellCommand(conn, cmd)
	if err != nil {
		slog.Error("压缩目录命令错误", "err_msg", err.Error(), "output", out)
		c.JSON(200, gin.H{"code": 4, "msg": "压缩目录错误", "data": out})
		return
	}

	c.JSON(200, gin.H{"code": 0, "msg": "压缩成功", "data": gin.H{"path": archivePath, "output": out}})
}

func archiveExtractCommand(archivePath string, dstPath string) (string, bool) {
	lowerName := strings.ToLower(path.Base(archivePath))
	archive := shellQuote(archivePath)
	dst := shellQuote(dstPath)

	switch {
	case strings.HasSuffix(lowerName, ".tar.gz") || strings.HasSuffix(lowerName, ".tgz"):
		return fmt.Sprintf("mkdir -p %s && tar -xzf %s -C %s", dst, archive, dst), true
	case strings.HasSuffix(lowerName, ".tar.bz2") || strings.HasSuffix(lowerName, ".tbz2"):
		return fmt.Sprintf("mkdir -p %s && tar -xjf %s -C %s", dst, archive, dst), true
	case strings.HasSuffix(lowerName, ".tar.xz") || strings.HasSuffix(lowerName, ".txz"):
		return fmt.Sprintf("mkdir -p %s && tar -xJf %s -C %s", dst, archive, dst), true
	case strings.HasSuffix(lowerName, ".tar"):
		return fmt.Sprintf("mkdir -p %s && tar -xf %s -C %s", dst, archive, dst), true
	case strings.HasSuffix(lowerName, ".zip"):
		return fmt.Sprintf("mkdir -p %s && unzip -o %s -d %s", dst, archive, dst), true
	default:
		return "", false
	}
}

// SftpExtractArchive sftp 解压压缩包
func SftpExtractArchive(c *gin.Context) {
	defer func() {
		if err := recover(); err != nil {
			c.JSON(200, gin.H{"code": 4, "msg": "解压错误"})
			return
		}
	}()

	type Body struct {
		SessionId string `form:"session_id" binding:"required,min=1,max=128" json:"session_id"`
		Path      string `form:"path" binding:"required,min=1,max=1024" json:"path"`
		DstPath   string `form:"dst_path" binding:"required,min=1,max=1024" json:"dst_path"`
	}

	var body Body
	if err := c.ShouldBind(&body); err != nil {
		slog.Error("绑定数据错误", "err_msg", err.Error())
		c.JSON(200, gin.H{"code": 1, "msg": "输入数据不合法"})
		return
	}
	conn, err := getSshConn(body.SessionId)
	if err != nil {
		slog.Error(err.Error())
		c.JSON(200, gin.H{"code": 2, "msg": err.Error()})
		return
	}

	cmd, ok := archiveExtractCommand(path.Clean(body.Path), path.Clean(body.DstPath))
	if !ok {
		c.JSON(200, gin.H{"code": 3, "msg": "暂不支持该压缩包格式"})
		return
	}

	out, err := execSftpShellCommand(conn, cmd)
	if err != nil {
		slog.Error("解压命令错误", "err_msg", err.Error(), "output", out)
		c.JSON(200, gin.H{"code": 4, "msg": "解压错误", "data": out})
		return
	}

	c.JSON(200, gin.H{"code": 0, "msg": "解压成功", "data": gin.H{"output": out}})
}
