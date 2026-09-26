const name: string = window.PackageR.Name || "packageR";
const disableExternal: boolean = window.PackageR.DisableExternal;
const baseURL: string = window.PackageR.BaseURL;
const staticURL: string = window.PackageR.StaticURL;
const recaptcha: string = window.PackageR.ReCaptcha;
const recaptchaKey: string = window.PackageR.ReCaptchaKey;
const version: string = window.PackageR.Version;
const logoURL = `${staticURL}/img/logo.svg`;
const loginPage: boolean = window.PackageR.LoginPage;
const theme: UserTheme = window.PackageR.Theme;
const enableThumbs: boolean = window.PackageR.EnableThumbs;
const resizePreview: boolean = window.PackageR.ResizePreview;
const enableExec: boolean = window.PackageR.EnableExec;
const tusSettings = window.PackageR.TusSettings;
const origin = window.location.origin;
const tusEndpoint = `/api/tus`;
const catalogPreviewURL = window.PackageR.CatalogPreviewURL;

export {
  name,
  disableExternal,
  baseURL,
  logoURL,
  recaptcha,
  recaptchaKey,
  version,
  loginPage,
  theme,
  enableThumbs,
  resizePreview,
  enableExec,
  tusSettings,
  origin,
  tusEndpoint,
  catalogPreviewURL,
};
