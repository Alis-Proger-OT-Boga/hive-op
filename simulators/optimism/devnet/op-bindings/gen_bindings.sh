#/bin/bash
set -eu

CONTRACTS_PATH="../contracts-bedrock/"


if [ "$#" -ne 2 ]; then
	echo "This script takes 2 arguments - CONTRACT_NAME PACKAGE"
	exit 1
fi


TYPE=$1
PACKAGE=$2

# Convert to lower case to respect golang package naming conventions
TYPE_LOWER=$(echo ${TYPE} | tr '[:upper:]' '[:lower:]')
FILENAME="${TYPE_LOWER}_deployed.go"


mkdir -p bin
TEMP=$(mktemp -d)

CWD=$(pwd)
# Build contracts
cd ${CONTRACTS_PATH}
forge build
forge inspect ${TYPE} abi > ${TEMP}/${TYPE}.abi
forge inspect ${TYPE} bytecode > ${TEMP}/${TYPE}.bin
forge inspect ${TYPE} deployedBytecode > ${CWD}/bin/${TYPE_LOWER}_deployed.hex

# Run ABIGEN
cd ${CWD}
abigen \
	--abi ${TEMP}/${TYPE}.abi \
	--bin ${TEMP}/${TYPE}.bin \
	--pkg ${PACKAGE} \
	--type ${TYPE} \
	--out ./${PACKAGE}/${TYPE_LOWER}.go
