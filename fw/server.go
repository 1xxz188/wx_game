package fw

type ServerType int32

const (
	ServerTypeLogic ServerType = 1
)

var ServerMapTypeToName = initMapTypeToName()
var ServerMapNameToType = initMapNameToType()

func initMapTypeToName() map[ServerType]string {
	m2 := make(map[ServerType]string)
	m2[ServerTypeLogic] = "logic_server"
	return m2
}

func initMapNameToType() map[string]ServerType {
	mName := make(map[string]ServerType)
	for svType, svName := range ServerMapTypeToName {
		mName[svName] = svType
	}
	return mName
}
