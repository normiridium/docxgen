package modifiers

var QrCodeFunc func(string, ...string) RawXML

// QrCode — вставляет QR-код по заданному значению прямо в документ.
//
// Пример использования:
//
//	{project.code|qrcode:`right`:`top`:`8%`:`5/5`:`border`}
//
// Формат:
//
//	{значение|qrcode:[mode]:[align]:[valign]:[crop%]:[margins]:[border]}
//
// Параметры (все необязательные, порядок не важен):
//
//   - mode — "anchor" (по умолчанию) или "inline"
//     Режим вставки: плавающий (anchor) или встроенный в текст (inline).
//
//   - align — "left", "center", "right"
//     Горизонтальное выравнивание для режима anchor (по умолчанию "right").
//
//   - valign — "top", "middle", "bottom"
//     Вертикальное выравнивание (по умолчанию "top").
//     "middle" — синоним "center".
//
//   - <N>mm — размер QR-кода в миллиметрах (по умолчанию 32 мм).
//
//   - <N>% — кроп (обрезка белых полей вокруг QR-кода), по умолчанию 4 %.
//
//   - margins — отступы от текста, в миллиметрах.
//     Форматы:
//     "5/5"         — верх/низ = 5 мм, лево/право = 5 мм;
//     "5/3/5/3"     — top/right/bottom/left отдельно;
//     "5/3/7"       — top, боковые, низ.
//
//   - border — флаг, добавляет тонкую чёрную рамку (≈ 0.5 pt) вокруг QR-кода.
//
// Возвращает:
//
//	Вставляемый XML-фрагмент <w:drawing> с сгенерированным QR-изображением.
//
// Совместимо с Microsoft Word, LibreOffice, OnlyOffice.
func QrCode(value string, opts ...string) RawXML {
	if QrCodeFunc == nil {
		return ""
	}
	return QrCodeFunc(value, opts...)
}
