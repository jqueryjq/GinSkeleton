package web

import (
	"github.com/gin-gonic/gin"
	"goskeleton/app/global/variable"
	"goskeleton/app/service/uploadFile"
)

type Upload struct {
}

//  文件上传是一个独立模块，给任何业务返回文件上传后的存储路径即可。
// 开始上传
func (u *Upload) Start(context *gin.Context) bool {
	save_path := variable.BASE_PATH + variable.UploadFileSavePath
	return uploadFile.Upload(context, save_path)
}