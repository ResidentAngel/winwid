//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	user32                = syscall.NewLazyDLL("user32.dll")
	dwmapi                = syscall.NewLazyDLL("dwmapi.dll")
	procFindWindowW       = user32.NewProc("FindWindowW")
	procGetWindowLongW    = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongW    = user32.NewProc("SetWindowLongPtrW")
	procSetWindowPos      = user32.NewProc("SetWindowPos")
	procIsIconic          = user32.NewProc("IsIconic")
	procShowWindow        = user32.NewProc("ShowWindow")
	procGetWindowRect     = user32.NewProc("GetWindowRect")
	procSystemParamsInfoW = user32.NewProc("SystemParametersInfoW")
	procDwmSetAttr        = dwmapi.NewProc("DwmSetWindowAttribute")
)

// RECT — структура координат окна в ядре Windows 4 поля по 32 бита = 16 байт
type RECT struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

const (
	GWL_STYLE        = -16
	GWL_EXSTYLE      = -20
	WS_CAPTION       = 0x00C00000 // стандартная шапка с крестиком
	WS_THICKFRAME    = 0x00040000 // рамка изменения размера
	WS_EX_TOOLWINDOW = 0x00000080
	WS_EX_APPWINDOW  = 0x00040000

	HWND_BOTTOM      = 1
	SWP_NOMOVE       = 0x0002
	SWP_NOSIZE       = 0x0001
	SWP_NOACTIVATE   = 0x0010
	SWP_FRAMECHANGED = 0x0020

	SW_SHOWNOACTIVATE = 4
	SPI_GETWORKAREA   = 0x0030

	DWMWA_WINDOW_CORNER_PREFERENCE = 33
	DWMWCP_DONOTROUND              = 1
	DWMWA_BORDER_COLOR             = 34
	DWMWA_COLOR_NONE               = 0xFFFFFFFE
)

func MakeWidgetWindow(title string) error {
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return err
	}

	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(titlePtr)))
	if hwnd == 0 {
		return syscall.EINVAL
	}

	// убираем из базового стиля окна шапку WS_CAPTION и теневую рамку WS_THICKFRAME
	styleIdx := GWL_STYLE
	style, _, _ := procGetWindowLongW.Call(hwnd, uintptr(styleIdx))
	newBaseStyle := style &^ (WS_CAPTION | WS_THICKFRAME)
	procSetWindowLongW.Call(hwnd, uintptr(styleIdx), newBaseStyle)

	// прячем из панели задач и Alt+Tab
	exStyleIdx := GWL_EXSTYLE
	exStyle, _, _ := procGetWindowLongW.Call(hwnd, uintptr(exStyleIdx))
	newExStyle := (exStyle | WS_EX_TOOLWINDOW) &^ WS_EX_APPWINDOW
	procSetWindowLongW.Call(hwnd, uintptr(exStyleIdx), newExStyle)

	// отключаем внешнюю обводку и тень углов Windows 11
	cornerPref := uint32(DWMWCP_DONOTROUND)
	procDwmSetAttr.Call(hwnd, DWMWA_WINDOW_CORNER_PREFERENCE, uintptr(unsafe.Pointer(&cornerPref)), unsafe.Sizeof(cornerPref))

	borderNone := uint32(DWMWA_COLOR_NONE)
	procDwmSetAttr.Call(hwnd, DWMWA_BORDER_COLOR, uintptr(unsafe.Pointer(&borderNone)), unsafe.Sizeof(borderNone))

	SendToDesktop(hwnd)
	return nil
}

func SendToDesktop(hwnd uintptr) {
	// если свернули через Win+D — возвращаем на рабочий стол
	isMinimized, _, _ := procIsIconic.Call(hwnd)
	if isMinimized != 0 {
		procShowWindow.Call(hwnd, SW_SHOWNOACTIVATE)
	}

	// проверяем границы экрана и не даем утащить виджет за монитор
	var winRect, workArea RECT
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&winRect)))
	procSystemParamsInfoW.Call(SPI_GETWORKAREA, 0, uintptr(unsafe.Pointer(&workArea)), 0)

	w := winRect.Right - winRect.Left
	h := winRect.Bottom - winRect.Top
	x := winRect.Left
	y := winRect.Top

	// ограничиваем по левому и правому краю экрана
	if x < workArea.Left {
		x = workArea.Left
	}
	if x+w > workArea.Right {
		x = workArea.Right - w
	}

	// ограничиваем по верхнему и нижнему краю
	if y < workArea.Top {
		y = workArea.Top
	}
	if y+h > workArea.Bottom {
		y = workArea.Bottom - h
	}

	// если координаты вышли за пределы экрана двигаем окно обратно в рамки убираем флаг SWP_NOMOVE
	if x != winRect.Left || y != winRect.Top {
		procSetWindowPos.Call(
			hwnd,
			uintptr(HWND_BOTTOM),
			uintptr(x), uintptr(y), 0, 0,
			SWP_NOSIZE|SWP_NOACTIVATE|SWP_FRAMECHANGED,
		)
		return
	}

	// иначе просто держим на нижнем слое рабочего стола
	procSetWindowPos.Call(
		hwnd,
		uintptr(HWND_BOTTOM),
		0, 0, 0, 0,
		SWP_NOMOVE|SWP_NOSIZE|SWP_NOACTIVATE|SWP_FRAMECHANGED,
	)
}
