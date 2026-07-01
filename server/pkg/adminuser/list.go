package adminuser

type List []*AdminUser

func (l List) IDs() IDList {
	if l == nil {
		return nil
	}
	ids := make(IDList, 0, len(l))
	for _, u := range l {
		if u != nil {
			ids = append(ids, u.ID())
		}
	}
	return ids
}
