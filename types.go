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
	case "int64", "Int64":
		return cast.ToInt64E(value)
	case "uInt8", "UInt8":
		return cast.ToUint8E(value)
	case "UInt16":
		return cast.ToUint16E(value)
	case "UInt32":
		return cast.ToUint32E(value)
	case "UInt64":
		return cast.ToUint64E(value)
	case "IPv4":
		ip := net.ParseIP(value.(string))
		if ip == nil {
			return nil, errors.New("无效的Ip地址:" + value.(string))
		}
		ip = ip.To4()
		if ip == nil {
			err := errors.New("不是有效的IPv4地址:" + value.(string))
			return nil, err
		}
		return ip, nil
	case "IPv6":
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
	case "Map(String, String)", "map[string]interface {}":
		if value == nil {
			return map[string]string{}, nil
		}
		if m, ok := value.(map[string]interface{}); ok {
			return convertMap(m), nil
		} else {
			if m, ok := value.(mapstr.M); ok {
				return convertCustomMap(m), nil
			}
			return map[string]string{}, errors.New("unsupported values:" + value.(string))
		}
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

func convertCustomMap(value mapstr.M) map[string]string {
	m2 := make(map[string]string, len(value))
	for k, v := range value {
		if strValue, ok := v.(string); ok {
			m2[k] = strValue
		} else {
			strValue = fmt.Sprintf("%v", v)
			m2[k] = strValue
		}
	}
	return m2
}
