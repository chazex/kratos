package selector

// 全局selector构建器
var globalSelector Builder

// GlobalSelector returns global selector builder.
func GlobalSelector() Builder {
	return globalSelector
}

// SetGlobalSelector set global selector builder.
func SetGlobalSelector(builder Builder) {
	globalSelector = builder
}
