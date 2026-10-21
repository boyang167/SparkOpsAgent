# 前端插件安装
### 依赖插件
```
    npm i monaco-editor-vue3@0.1.6 js-yaml@4.1.0  \
          vue-chartjs@4.1.1 \
          xterm@4.19.0 \
          xterm-addon-fit@0.5.0 \
          js-base64@^3.7.3 \
          asciinema-player@^3.6.1 \
          vue3-tree-org@^4.2.2 \
          monaco-editor@^0.48.0 \
          moment@^2.30.1
```
###  兼容 monaco-editor 编辑器修改全局默认样式
```
gin-vue-admin-2.7.2/web/src/style/reset.scss

注解 font-family 这个全局样式

td {
  border: none;
  
  //font-family: "Helvetica Neue", Helvetica, "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", "微软雅黑", Arial, sans-serif;
  font-size: 14px;
  margin: 0px;
  padding: 0px;
}

```

