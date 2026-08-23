# 多阶段构建优化前端镜像

ARG PREVIOUS_FRONTEND_IMAGE=nginx:alpine

# 保留上一版本当前使用的静态资源，让发布时已经打开的页面仍能完成懒加载。
FROM ${PREVIOUS_FRONTEND_IMAGE} AS previous

RUN mkdir -p /tmp/previous-assets && \
    if [ -s /usr/share/nginx/html/current-assets.txt ]; then \
      cd /usr/share/nginx/html && \
      while IFS= read -r asset; do \
        case "$asset" in \
          assets/*) [ ! -f "$asset" ] || cp "$asset" /tmp/previous-assets/ ;; \
        esac; \
      done < current-assets.txt; \
    elif [ -d /usr/share/nginx/html/assets ]; then \
      find /usr/share/nginx/html/assets -maxdepth 1 -type f ! -name '*.map' \
        -exec cp '{}' /tmp/previous-assets/ \;; \
    fi

# ========================================
# 阶段一：构建器
# ========================================
FROM node:18-alpine AS builder

WORKDIR /app

# 复制依赖文件
COPY package*.json ./

# 安装依赖
RUN npm install

# 复制源代码
COPY . .

# 构建前端
RUN npm run build && \
    find dist/assets -maxdepth 1 -type f ! -name '*.map' \
      | sed 's#^dist/##' \
      | sort > dist/current-assets.txt

# ========================================
# 阶段二：生产镜像
# ========================================
FROM nginx:alpine

# 复制 nginx 配置
COPY nginx.conf /etc/nginx/conf.d/default.conf

# 先复制上一版本资源，再让本次构建产物覆盖同名文件。
COPY --from=previous /tmp/previous-assets/ /usr/share/nginx/html/assets/
COPY --from=builder /app/dist /usr/share/nginx/html

# 暴露端口
EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]
