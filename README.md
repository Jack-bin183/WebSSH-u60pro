## WebSSH

---
### 后端编译
>  必须使用golang 1.21以上版本
* cd WebSSH/gossh
* go build
* ./gossh
>  打开链接 http://127.0.0.1:8899/ 开始享用吧,第一次需要初始化
<br/>

### 前端开发
* cd WebSSH/webssh
* npm install
* npm run dev
>  打开链接 http://127.0.0.1:3000/  支持热重载

### 前端编译

* cd WebSSH/webssh
* npm install
* npm run build

### 打包二进制

npm run build 执行之后移动编译产物到后端 webroot 文件夹，一条代码完成 

* rsync -a --delete dist/ ../gossh/webroot/
* cd ../gossh
* CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o webssh

然后使用 upx 压缩一下（可选）
* upx --best --lzma webssh

* 一条命令完成（VERSION 自行修改）

npm run build \
&& mkdir -p ../gossh/webroot \
&& rsync -a --delete dist/ ../gossh/webroot/ \
&& cd ../gossh \
&& VERSION=20260514_1414 \
&& CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X main.version=${VERSION}"  -o webssh \
&& upx --best --lzma webssh

然后把 webssh 上传即可

通常需要先 cd 到目录，再使用 ./webssh 运行程序，因为 SqlLite 数据库文件会存储在运行目录