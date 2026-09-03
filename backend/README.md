# Blog Backend

### Go版本

* 推荐 go1.23 及以上

### 安装必要工具

* 安装proto-gen-http等工具
```bash
make init
```

### 新增api
编辑proto文件，编写所需要的api接口，然后执行命令
```bash
make api
```

### 安装依赖
```bash
make gen
```

### 修改配置

```
cp .env.example .env
```

### 运行
```bash
make run
```

#### 运行脚本

```bash
make runjob

OR

go run main.go job blog
```
### 打包

* 你的deploy目录下脚本

# links
- [Leo框架](https://leo.qmniu.com/)
