package _89

import (
	"errors"
	"fmt"
	"github.com/Xhofe/alist/conf"
	"github.com/Xhofe/alist/drivers/base"
	"github.com/Xhofe/alist/model"
	"github.com/Xhofe/alist/utils"
	"github.com/gin-gonic/gin"
	jsoniter "github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"
	"path/filepath"
)

type Cloud189 struct{}

func (driver Cloud189) Config() base.DriverConfig {
	return base.DriverConfig{
		Name: "189Cloud",
	}
}

func (driver Cloud189) Items() []base.Item {
	return []base.Item{
		{
			Name:        "username",
			Label:       "username",
			Type:        base.TypeString,
			Required:    true,
			Description: "account username/phone number",
		},
		{
			Name:        "password",
			Label:       "password",
			Type:        base.TypeString,
			Required:    true,
			Description: "account password",
		},
		{
			Name:     "root_folder",
			Label:    "root folder file_id",
			Type:     base.TypeString,
			Required: true,
		},
		//{
		//	Name:     "internal_type",
		//	Label:    "189cloud type",
		//	Type:     base.TypeSelect,
		//	Required: true,
		//	Values:   "Personal,Family",
		//},
		//{
		//	Name:  "site_id",
		//	Label: "family id",
		//	Type:  base.TypeString,
		//},
		{
			Name:     "order_by",
			Label:    "order_by",
			Type:     base.TypeSelect,
			Values:   "name,size,lastOpTime,createdDate",
			Required: true,
		},
		{
			Name:     "order_direction",
			Label:    "desc",
			Type:     base.TypeSelect,
			Values:   "true,false",
			Required: true,
		},
	}
}

func (driver Cloud189) Save(account *model.Account, old *model.Account) error {
	if old != nil {
		delete(client189Map, old.Name)
	}
	if account == nil {
		return nil
	}
	if err := driver.Login(account); err != nil {
		account.Status = err.Error()
		_ = model.SaveAccount(account)
		return err
	}
	sessionKey, err := driver.GetSessionKey(account)
	if err != nil {
		account.Status = err.Error()
	} else {
		account.Status = "work"
		account.DriveId = sessionKey
	}
	_ = model.SaveAccount(account)
	return err
}

func (driver Cloud189) File(path string, account *model.Account) (*model.File, error) {
	path = utils.ParsePath(path)
	if path == "/" {
		return &model.File{
			Id:        account.RootFolder,
			Name:      account.Name,
			Size:      0,
			Type:      conf.FOLDER,
			Driver:    driver.Config().Name,
			UpdatedAt: account.UpdatedAt,
		}, nil
	}
	dir, name := filepath.Split(path)
	files, err := driver.Files(dir, account)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if file.Name == name {
			return &file, nil
		}
	}
	return nil, base.ErrPathNotFound
}

func (driver Cloud189) Files(path string, account *model.Account) ([]model.File, error) {
	path = utils.ParsePath(path)
	var rawFiles []Cloud189File
	cache, err := base.GetCache(path, account)
	if err == nil {
		rawFiles, _ = cache.([]Cloud189File)
	} else {
		file, err := driver.File(path, account)
		if err != nil {
			return nil, err
		}
		rawFiles, err = driver.GetFiles(file.Id, account)
		if err != nil {
			return nil, err
		}
		if len(rawFiles) > 0 {
			_ = base.SetCache(path, rawFiles, account)
		}
	}
	files := make([]model.File, 0)
	for _, file := range rawFiles {
		files = append(files, *driver.FormatFile(&file))
	}
	return files, nil
}

func (driver Cloud189) Link(args base.Args, account *model.Account) (*base.Link, error) {
	file, err := driver.File(utils.ParsePath(args.Path), account)
	if err != nil {
		return nil, err
	}
	if file.Type == conf.FOLDER {
		return nil, base.ErrNotFile
	}
	var resp Cloud189Down
	u := "https://cloud.189.cn/api/open/file/getFileDownloadUrl.action"
	body, err := driver.Request(u, base.Get, map[string]string{
		"fileId": file.Id,
	}, nil, nil, account)
	if err != nil {
		return nil, err
	}
	err = utils.Json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}
	if resp.ResCode != 0 {
		return nil, fmt.Errorf(resp.ResMessage)
	}
	res, err := base.NoRedirectClient.R().Get(resp.FileDownloadUrl)
	if err != nil {
		return nil, err
	}
	link := base.Link{}
	if res.StatusCode() == 302 {
		link.Url = res.Header().Get("location")
	} else {
		link.Url = resp.FileDownloadUrl
	}
	return &link, nil
}

func (driver Cloud189) Path(path string, account *model.Account) (*model.File, []model.File, error) {
	path = utils.ParsePath(path)
	log.Debugf("189 path: %s", path)
	file, err := driver.File(path, account)
	if err != nil {
		return nil, nil, err
	}
	if !file.IsDir() {
		return file, nil, nil
	}
	files, err := driver.Files(path, account)
	if err != nil {
		return nil, nil, err
	}
	return nil, files, nil
}

func (driver Cloud189) Proxy(ctx *gin.Context, account *model.Account) {
	ctx.Request.Header.Del("Origin")
}

func (driver Cloud189) Preview(path string, account *model.Account) (interface{}, error) {
	return nil, base.ErrNotSupport
}

func (driver Cloud189) MakeDir(path string, account *model.Account) error {
	dir, name := filepath.Split(path)
	parent, err := driver.File(dir, account)
	if err != nil {
		return err
	}
	if !parent.IsDir() {
		return base.ErrNotFolder
	}
	form := map[string]string{
		"parentFolderId": parent.Id,
		"folderName":     name,
	}
	_, err = driver.Request("https://cloud.189.cn/api/open/file/createFolder.action", base.Post, nil, form, nil, account)
	return err
}

func (driver Cloud189) Move(src string, dst string, account *model.Account) error {
	dstDir, dstName := filepath.Split(dst)
	srcFile, err := driver.File(src, account)
	if err != nil {
		return err
	}
	dstDirFile, err := driver.File(dstDir, account)
	if err != nil {
		return err
	}
	isFolder := 0
	if srcFile.IsDir() {
		isFolder = 1
	}
	taskInfos := []base.Json{
		{
			"fileId":   srcFile.Id,
			"fileName": dstName,
			"isFolder": isFolder,
		},
	}
	taskInfosBytes, err := utils.Json.Marshal(taskInfos)
	if err != nil {
		return err
	}
	form := map[string]string{
		"type":           "MOVE",
		"targetFolderId": dstDirFile.Id,
		"taskInfos":      string(taskInfosBytes),
	}
	_, err = driver.Request("https://cloud.189.cn/api/open/batch/createBatchTask.action", base.Post, nil, form, nil, account)
	return err
}

func (driver Cloud189) Rename(src string, dst string, account *model.Account) error {
	_, dstName := filepath.Split(dst)
	srcFile, err := driver.File(src, account)
	if err != nil {
		return err
	}
	url := "https://cloud.189.cn/api/open/file/renameFile.action"
	idKey := "fileId"
	nameKey := "destFileName"
	if srcFile.IsDir() {
		url = "https://cloud.189.cn/api/open/file/renameFolder.action"
		idKey = "folderId"
		nameKey = "destFolderName"
	}
	form := map[string]string{
		idKey:   srcFile.Id,
		nameKey: dstName,
	}
	_, err = driver.Request(url, base.Post, nil, form, nil, account)
	return err
}

func (driver Cloud189) Copy(src string, dst string, account *model.Account) error {
	dstDir, dstName := filepath.Split(dst)
	srcFile, err := driver.File(src, account)
	if err != nil {
		return err
	}
	dstDirFile, err := driver.File(dstDir, account)
	if err != nil {
		return err
	}
	isFolder := 0
	if srcFile.IsDir() {
		isFolder = 1
	}
	taskInfos := []base.Json{
		{
			"fileId":   srcFile.Id,
			"fileName": dstName,
			"isFolder": isFolder,
		},
	}
	taskInfosBytes, err := utils.Json.Marshal(taskInfos)
	if err != nil {
		return err
	}
	form := map[string]string{
		"type":           "COPY",
		"targetFolderId": dstDirFile.Id,
		"taskInfos":      string(taskInfosBytes),
	}
	_, err = driver.Request("https://cloud.189.cn/api/open/batch/createBatchTask.action", base.Post, nil, form, nil, account)
	return err
}

func (driver Cloud189) Delete(path string, account *model.Account) error {
	path = utils.ParsePath(path)
	file, err := driver.File(path, account)
	if err != nil {
		return err
	}
	isFolder := 0
	if file.IsDir() {
		isFolder = 1
	}
	taskInfos := []base.Json{
		{
			"fileId":   file.Id,
			"fileName": file.Name,
			"isFolder": isFolder,
		},
	}
	taskInfosBytes, err := utils.Json.Marshal(taskInfos)
	if err != nil {
		return err
	}
	form := map[string]string{
		"type":           "DELETE",
		"targetFolderId": "",
		"taskInfos":      string(taskInfosBytes),
	}
	_, err = driver.Request("https://cloud.189.cn/api/open/batch/createBatchTask.action", base.Post, nil, form, nil, account)
	return err
}

func (driver Cloud189) Upload(file *model.FileStream, account *model.Account) error {
	//return base.ErrNotImplement
	if file == nil {
		return base.ErrEmptyFile
	}
	client, ok := client189Map[account.Name]
	if !ok {
		return fmt.Errorf("can't find [%s] client", account.Name)
	}
	parentFile, err := driver.File(file.ParentPath, account)
	if err != nil {
		return err
	}
	// api refer to PanIndex
	res, err := client.R().SetMultipartFormData(map[string]string{
		"parentId":   parentFile.Id,
		"sessionKey": account.DriveId,
		"opertype":   "1",
		"fname":      file.GetFileName(),
	}).SetMultipartField("Filedata", file.GetFileName(), file.GetMIMEType(), file).Post("https://hb02.upload.cloud.189.cn/v1/DCIWebUploadAction")
	if err != nil {
		return err
	}
	if jsoniter.Get(res.Body(), "MD5").ToString() != "" {
		return nil
	}
	log.Debugf(res.String())
	return errors.New(res.String())
}

var _ base.Driver = (*Cloud189)(nil)
