FROM golang:1.20-bullseye

# Set an environment variable to specify the location of the application
ENV APP_LOCATION /app
ENV GOPROXY=https://goproxy.cn,direct

# Create the app directory and set it as the working directory
RUN mkdir -p $APP_LOCATION
WORKDIR $APP_LOCATION

# Copy the source code into the container
COPY . $APP_LOCATION

# Update mirror
# RUN sed -i -e 's/http:\/\/[^\/]*/http:\/\/mirrors.ustc.edu.cn/g' /etc/apt/sources.list \
#     && apt update && apt install -y iputils-ping netcat iproute2 net-tools procps bsdmainutils vim tree fio lsof sudo wget gnupg\
#     && rm -rf /var/lib/apt/lists/*

# Build the Go program
RUN make hugo
