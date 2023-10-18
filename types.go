package clickhouse

import (
	"errors"
	"fmt"
	"github.com/elastic/elastic-agent-libs/mapstr"
	"github.com/google/uuid"
	"github.com/spf13/cast"
	"net"
)

func toClickhouseType(value interface{}, valueType string) (interface{}, error) {
	switch valueType {
	case "float32", "Float32":
		return cast.ToFloat32E(value)
	case "float64", "Float64":
		return cast.ToFloat64E(value)
	case "int8", "Int8":
		return cast.ToInt8E(value)
	case "int16", "Int16":
		return cast.ToInt16E(value)
	case "int32", "Int32":
		return cast.ToInt32E(value)
	case "Int64":
		return cast.ToInt64E(value)
	case "uInt8", "UInt8":
		return cast.ToUint8E(value)
	case "UInt16":
		return cast.ToUint16E(value)
	case "UInt32":
		return cast.ToUint32E(value)
	case "int64", "UInt64":
		return cast.ToUint64E(value)
	case "ipv4", "IPv4", "IPv6":
		ip, _, err := net.ParseCIDR(value.(string))
		return ip, err
	case "Bool", "Boolean":
		return cast.ToBoolE(value)
	case "Date", "Date32", "DateTime":
		return cast.ToTimeE(value)
	case "UUID":
		return uuid.Parse(value.(string))
	case "string", "String":
		return cast.ToStringE(value)
	case "map[string]interface {}":
		if value == nil {
			return nil, nil
		}
		return convertMap(value.(map[string]interface{})), nil
	case "mapstr.M":
		var val = value.(mapstr.M)
		if len(val) == 0 {
			return nil, nil
		}
		m2 := make(map[string]interface{}, len(val))
		for k, v := range val {
			m2[k] = v
		}
		return convertMap(m2), nil
	default:
		return "", errors.New("unsupported type:" + valueType)
	}
}

func convertMap(originalMap map[string]interface{}) map[string]string {
	resultMap := make(map[string]string)

	for key, value := range originalMap {
		if strValue, ok := value.(string); ok {
			resultMap[key] = strValue
		} else {
			strValue = fmt.Sprintf("%v", value)
			resultMap[key] = strValue
		}
	}

	return resultMap
}
