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

func ProcessExtensions(value interface{}, extensions string, extHandler ExtensionHandler) (interface{}, error) {
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
	if len(extensionList) > 0 {
		return extHandler.Process(extensionList, value)
	}
	return value, nil
}
