package yext

type ExtensionHandler interface {
	Process(extensions []ExtensionParams, inputData ...interface{}) (outputData interface{}, err error)
	IsExtensionHandler() bool
	IsExtensionSupported(extension ExtensionParams) bool
}

type ExtensionParams struct {
	Keyword, Argument string
}
