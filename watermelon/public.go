package watermelon

import "wx_game/msg"

func EqualSnapshot(a, b *msg.WatermelonRecordSnapshot) bool {
	if a == nil || b == nil {
		return a == b
	}
	if len(a.Records) != len(b.Records) {
		return false
	}
	for i := range a.Records {
		if !EqualEntity(a.Records[i], b.Records[i]) {
			return false
		}
	}
	return true
}

func EqualEntity(a, b *msg.WatermelonEntity) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Id == b.Id && a.Level == b.Level
}
