package yext

import "strings"

type ExtensionHandler interface {
	Process(extensions []ExtensionParams, inputData ...interface{}) (outputData interface{}, err error)
	IsExtensionHandler() bool
	IsExtensionSupported(extension ExtensionParams) bool
}

type ExtensionParams struct {
	Keyword, Argument string
}

func GetSupportedExtensions(extensions string, extHandler ExtensionHandler) []ExtensionParams {
	extSlice := strings.Split(extensions, ";")
	var extensionList []ExtensionParams = []ExtensionParams{}
	for _, ext := range extSlice {
		nameAndArg := strings.Split(ext, ",")
		keyword := nameAndArg[0]
		argument := ""
		if len(nameAndArg) == 2 {
			argument = nameAndArg[1]
		}
		extension := ExtensionParams{Keyword: keyword, Argument: argument}
		if extHandler.IsExtensionSupported(extension) {
			extensionList = append(extensionList, extension)
		}
	}
	return extensionList
}
