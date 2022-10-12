#!/bin/bash
PILOTGO_VERSION="v0.0.1"

echo "thanks for choosing PilotGo"

# 判断是否安装了NodeJS
echo "checking frontend compile tools..."
if ! type node >/dev/null 2>&1; then
    echo "no nodejs detected, please install nodejs >= 14.0"
    exit -1
else
    NodeJS=`node -v | grep -oP '\d*\.\d*.\d+'`
    if [ ${NodeJS:0:2} -lt 14 ]; then
        echo "error: your nodejs is too old, please upgrade to v14.0 or newer"
        exit -1
    fi

    # 判断是否安装了NPM
    if ! type npm >/dev/null 2>&1; then
        echo "error: your npm is too old, please upgrade to v6.0 or newer"
        exit -1;
    fi
fi
echo "ok"

# 判断是否安装了golang
echo "Checking backend compile tools..."
if ! type go >/dev/null 2>&1; then
    echo "no golang detected, please install golang >= 1.17.0"
    exit -1
else
    GoLang=`go version |awk '{print $3}' | grep -oP '\d*\.\d*.\d+'`
    if [ ${GoLang: 2: 2} -lt 17 ]; then
        echo "error: your golang is too old, please upgrade to v1.17.0 or newer"
        exit -1
    fi
fi
echo "ok"

function build_frontend() {
    echo "dowoloading frontend libraries, please wait..."
    yarn install
    echo "compiling frontend, please wait..."
    yarn run build
    if [ "$?" != "0" ]; then
        echo 'error: build frontend failed, please check the error'
        exit -1
    fi

    # move frontend binary files to resource dir
    cp ./dist/index.html ./resource/index.html
    cp -r ./dist/static/* ./resource/
}

function build_and_pack() {
    # must provide arch parameter(amd64, arm64 or i386, must meet GOARCH requires)

    echo "cleanning tmp directory..."
    rm -rf ./out/${1}

    version_path="./out/${1}/pilotgo-${PILOTGO_VERSION}/"

    echo "building server for ${1}..."
    mkdir -p ${version_path}/server
    CGO_ENABLED=0 GOOS=linux GOARCH=${1} go build -o ${version_path}/server/pilotgo-server ./pkg/app/server/main.go
    cp config_server.yaml.templete ${version_path}/server/config_server.yaml
    cp alert.rules.templete ${version_path}/server/alert.rules

    echo "building agent for ${1}..."
    mkdir -p ${version_path}/agent
    CGO_ENABLED=0 GOOS=linux GOARCH=${1} go build -o ${version_path}/agent/pilotgo-agent pkg/app/agent/main.go
    cp config_agent.yaml.templete ${version_path}/agent/config_agent.yaml

    echo "adding scripts..."
    cp ./scripts/shell/install_server.sh ${version_path}/server/
    cp ./scripts/shell/install_agent.sh ${version_path}/agent/

    echo "compressing files..."
    tar -czf ./out/pilotgo-${PILOTGO_VERSION}-${1}.tar.gz -C ./out/${1} .
}

function clean() {
    rm -rf ./out
}

build_frontend

build_and_pack amd64
build_and_pack arm64

echo "done"
