package cfg

import (
	"encoding/json"
	"github.com/donnie4w/go-logger/logger"
	"io/ioutil"
	cfg "wx_game/cfg/code"
)

var tables *cfg.Tables
var dataDir = "./cfg/data/"

func loader(file string) ([]map[string]interface{}, error) {
	if bytes, err := ioutil.ReadFile(dataDir + file + ".json"); err != nil {
		return nil, err
	} else {
		jsonData := make([]map[string]interface{}, 0)
		if err = json.Unmarshal(bytes, &jsonData); err != nil {
			return nil, err
		}
		return jsonData, nil
	}
}

func Init() error {
	logger.Info("load cfg...")
	t, err := cfg.NewTables(loader)
	if err != nil {
		return err
	}
	tables = t
	logger.Info("load cfg successful")
	logger.Debug("table data: ", tables.TbWaterMelonConfig.Get(1).InitFruit)
	return nil
}

func Tables() *cfg.Tables {
	return tables
}

func SetDataDir(path string) {
	dataDir = path
}
