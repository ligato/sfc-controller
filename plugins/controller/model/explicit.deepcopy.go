package controller

// DeepCopyInto is a deepcopy function, copying the receiver, writing into out. in must be non-nil.
// Explicitly defined to provide for deep copy of vpp-agent types (l3.Route)
func (in *L3VRFRoute) DeepCopyInto(out *L3VRFRoute) {
	*out = *in
	if in.GetVpp() != nil {
		out = &L3VRFRoute{
			Vpp: in.GetVpp(),
		}
	}

	if in.GetLinux() != nil {
		out = &L3VRFRoute{
			Linux: in.GetLinux(),
		}
	}
	out.XXX_NoUnkeyedLiteral = in.XXX_NoUnkeyedLiteral
	if in.XXX_unrecognized != nil {
		in, out := &in.XXX_unrecognized, &out.XXX_unrecognized
		*out = make([]byte, len(*in))
		copy(*out, *in)
	}
	return
}
