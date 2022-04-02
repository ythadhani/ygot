package ygot

import (
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

type NotificationWithAdds struct {
	*gnmipb.Notification
	Addition []*gnmipb.Update
}

func (n *NotificationWithAdds) GetAddition() []*gnmipb.Update {
	if n != nil {
		return n.Addition
	}
	return nil
}
