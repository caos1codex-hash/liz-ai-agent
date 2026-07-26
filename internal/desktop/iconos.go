package desktop

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Helpers de iconos. Centraliza los imports de theme para que los componentes
// de UI no tengan que depender directamente del paquete theme. Si en el futuro
// se quiere usar un set de iconos custom, basta con cambiar este archivo.

func theme_iconDelete() fyne.Resource       { return theme.DeleteIcon() }
func theme_iconRefresh() fyne.Resource      { return theme.ViewRefreshIcon() }
func theme_iconChat() fyne.Resource         { return theme.MailComposeIcon() }
func theme_iconPlus() fyne.Resource         { return theme.ContentAddIcon() }
func theme_iconSend() fyne.Resource         { return theme.MailSendIcon() }
func theme_iconSearch() fyne.Resource       { return theme.SearchIcon() }
func theme_iconSettings() fyne.Resource     { return theme.SettingsIcon() }
func theme_iconSun() fyne.Resource          { return theme.VisibilityIcon() }
func theme_iconMoon() fyne.Resource         { return theme.VisibilityOffIcon() }
func theme_iconComputer() fyne.Resource     { return theme.ComputerIcon() }
func theme_iconStorage() fyne.Resource      { return theme.StorageIcon() }
func theme_iconInfo() fyne.Resource         { return theme.InfoIcon() }
func theme_iconWarning() fyne.Resource      { return theme.WarningIcon() }
func theme_iconError() fyne.Resource        { return theme.ErrorIcon() }
func theme_iconCheck() fyne.Resource        { return theme.ConfirmIcon() }
func theme_iconMenu() fyne.Resource         { return theme.MenuIcon() }
func theme_iconClose() fyne.Resource        { return theme.CancelIcon() }
func theme_iconHome() fyne.Resource         { return theme.HomeIcon() }
func theme_iconDocument() fyne.Resource     { return theme.DocumentIcon() }
func theme_iconMoreVertical() fyne.Resource { return theme.MoreVerticalIcon() }
