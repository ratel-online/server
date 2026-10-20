package Milky_go_sdk

import "encoding/json"

type (
	RawMessageElement struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}

	IMessageElement interface {
		Type() MessageElementType
		MarshalJSON() ([]byte, error)
	}

	MessageElementType string
)

// Copied from sealdice/sealdice-core
const (
	Text       MessageElementType = "text"        // 文本
	At         MessageElementType = "mention"     // 艾特
	AtAll      MessageElementType = "mention_all" // 艾特全体
	Video      MessageElementType = "video"       // 视频
	Image      MessageElementType = "image"       // 图片
	Reply      MessageElementType = "reply"       // 回复
	Record     MessageElementType = "record"      // 语音
	Face       MessageElementType = "face"        // 表情
	Forward    MessageElementType = "forward"     // 转发
	MarketFace MessageElementType = "market_face" // 市场表情
	LightApp   MessageElementType = "light_app"   // 小程序
	XML        MessageElementType = "xml"         // XML
)

const maxFileSize = 1024 * 1024 * 50 // 50MB

// Message Elements. Letter I means Incoming, O means Outgoing (Sending)

type TextElement struct {
	// I & O
	Text string `json:"text"`
}

func (t *TextElement) Type() MessageElementType {
	return Text
}

func (t *TextElement) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Type MessageElementType `json:"type"`
		Data struct {
			Text string `json:"text"` // 文本内容
		} `json:"data"`
	}{
		Type: t.Type(),
		Data: struct {
			Text string `json:"text"` // 文本内容
		}{
			Text: t.Text,
		},
	})
}

type AtElement struct {
	// I & O
	UserID int64 `json:"user_id"`
}

func (t *AtElement) Type() MessageElementType {
	return At
}

func (t *AtElement) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Type MessageElementType `json:"type"`
		Data struct {
			UserID int64 `json:"user_id"`
		} `json:"data"`
	}{
		Type: t.Type(),
		Data: struct {
			UserID int64 `json:"user_id"`
		}{
			UserID: t.UserID,
		},
	})
}

type AtAllElement struct{}

func (t *AtAllElement) Type() MessageElementType {
	return AtAll
}

func (t *AtAllElement) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Type MessageElementType `json:"type"`
		Data struct{}           `json:"data"`
	}{
		Type: t.Type(),
		Data: struct{}{},
	})
}

type ReplyElement struct {
	// I & O
	MessageSeq int64 `json:"message_seq"` // 回复的目标消息ID
}

func (t *ReplyElement) Type() MessageElementType {
	return Reply
}

func (t *ReplyElement) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Type MessageElementType `json:"type"`
		Data struct {
			MessageSeq int64 `json:"message_seq"`
		} `json:"data"`
	}{
		Type: t.Type(),
		Data: struct {
			MessageSeq int64 `json:"message_seq"`
		}{
			MessageSeq: t.MessageSeq,
		},
	})
}

type ImageElement struct {
	// I & O
	Summary string `json:"summary"`  // 图片摘要
	SubType string `json:"sub_type"` // 图片子类型
	// O
	URI string `json:"uri,omitempty"` // 图片URI
	// I
	ResourceID string `json:"resource_id"` // 图片资源ID
	TempURL    string `json:"temp_url"`
	Width      int32  `json:"width"`
	Height     int32  `json:"height"`
}

func (l *ImageElement) Type() MessageElementType {
	return Image
}

func (l *ImageElement) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Type MessageElementType `json:"type"`
		Data struct {
			URI     string `json:"uri"`
			Summary string `json:"summary"`
			SubType string `json:"sub_type"`
		} `json:"data"`
	}{
		Type: l.Type(),
		Data: struct {
			URI     string `json:"uri"`
			Summary string `json:"summary"`
			SubType string `json:"sub_type"`
		}{
			URI:     l.URI,
			Summary: l.Summary,
			SubType: l.SubType,
		},
	})
}

type RecordElement struct {
	// O
	URI string `json:"uri,omitempty"` // 语音URI
	// I
	ResourceID string `json:"resource_id"` // 资源ID
	TempURL    string `json:"temp_url"`
	Duration   int32  `json:"duration"`
}

func (r *RecordElement) Type() MessageElementType {
	return Record
}

func (r *RecordElement) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Type MessageElementType `json:"type"`
		Data struct {
			URI string `json:"uri"`
		} `json:"data"`
	}{
		Type: r.Type(),
		Data: struct {
			URI string `json:"uri"`
		}{
			URI: r.URI,
		},
	})
}

type VideoElement struct {
	// O
	URI      string `json:"uri,omitempty"`
	ThumbURI string `json:"thumb_uri,omitempty"`
	// I
	ResourceID string `json:"resource_id"`
	TempURL    string `json:"temp_url"`
	Width      int32  `json:"width"`
	Height     int32  `json:"height"`
	Duration   int32  `json:"duration"`
}

func (v *VideoElement) Type() MessageElementType {
	return Video
}

func (v *VideoElement) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Type MessageElementType `json:"type"`
		Data struct {
			URI      string `json:"uri"`
			ThumbURI string `json:"thumb_uri"`
		} `json:"data"`
	}{
		Type: v.Type(),
		Data: struct {
			URI      string `json:"uri"`
			ThumbURI string `json:"thumb_uri"`
		}{
			URI:      v.URI,
			ThumbURI: v.ThumbURI,
		},
	})
}

type FaceElement struct {
	// I & O
	FaceID string `json:"face_id"`
}

func (f *FaceElement) Type() MessageElementType {
	return Face
}

func (f *FaceElement) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Type MessageElementType `json:"type"`
		Data struct {
			FaceID string `json:"face_id"`
		} `json:"data"`
	}{
		Type: f.Type(),
		Data: struct {
			FaceID string `json:"face_id"`
		}{
			FaceID: f.FaceID,
		},
	})
}

type OutgoingForwardedMessage struct {
	UserID   int64             `json:"user_id"`
	Name     string            `json:"name"`     // 转发者名称
	Segments []IMessageElement `json:"segments"` // 转发的消息内容
}

type ForwardElement struct {
	// I
	ForwardID string `json:"forward_id"` // 转发消息ID
	// O
	Messages []OutgoingForwardedMessage `json:"messages"`
}

func (f *ForwardElement) Type() MessageElementType {
	return Forward
}

func (f *ForwardElement) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Type MessageElementType `json:"type"`
		Data struct {
			Messages []OutgoingForwardedMessage `json:"messages"`
		} `json:"data"`
	}{
		Type: f.Type(),
		Data: struct {
			Messages []OutgoingForwardedMessage `json:"messages"`
		}{
			Messages: f.Messages,
		},
	})
}

// Incoming Only

type MarketFaceElement struct {
	URL string `json:"url"`
}

func (m *MarketFaceElement) Type() MessageElementType {
	return MarketFace
}

func (m *MarketFaceElement) MarshalJSON() ([]byte, error) {
	return json.Marshal(m)
}

type LightAppElement struct {
	AppName     string `json:"app_name"`
	JSONPayload string `json:"json_payload"`
}

func (l *LightAppElement) Type() MessageElementType {
	return LightApp
}

func (l *LightAppElement) MarshalJSON() ([]byte, error) {
	return json.Marshal(l)
}

type XmlElement struct {
	XML string `json:"xml"`
}

func (x *XmlElement) Type() MessageElementType {
	return XML
}

func (x *XmlElement) MarshalJSON() ([]byte, error) {
	return json.Marshal(x)
}
