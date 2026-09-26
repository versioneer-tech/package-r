const name: string = window.PackageR.Name || "packageR";
const disableExternal: boolean = window.PackageR.DisableExternal;
const disableUsedPercentage: boolean = window.PackageR.DisableUsedPercentage;
const baseURL: string = window.PackageR.BaseURL;
const staticURL: string = window.PackageR.StaticURL;
const recaptcha: string = window.PackageR.ReCaptcha;
const recaptchaKey: string = window.PackageR.ReCaptchaKey;
const signup: boolean = window.PackageR.Signup;
const version: string = window.PackageR.Version;
const logoURL = `${staticURL}/img/logo.svg`;
const noAuth: boolean = window.PackageR.NoAuth;
const authMethod = window.PackageR.AuthMethod;
const loginPage: boolean = window.PackageR.LoginPage;
const theme: UserTheme = window.PackageR.Theme;
const enableThumbs: boolean = window.PackageR.EnableThumbs;
const resizePreview: boolean = window.PackageR.ResizePreview;
const enableExec: boolean = window.PackageR.EnableExec;
const tusSettings = window.PackageR.TusSettings;
const origin = window.location.origin;
const tusEndpoint = `/api/tus`;
const shareLinkDefaultHash = window.PackageR.ShareLinkDefaultHash;
const catalogDefaultName = window.PackageR.CatalogDefaultName;
const catalogPreviewURL = window.PackageR.CatalogPreviewURL;

export {
  name,
  disableExternal,
  disableUsedPercentage,
  baseURL,
  logoURL,
  recaptcha,
  recaptchaKey,
  signup,
  version,
  noAuth,
  authMethod,
  loginPage,
  theme,
  enableThumbs,
  resizePreview,
  enableExec,
  tusSettings,
  origin,
  tusEndpoint,
  shareLinkDefaultHash,
  catalogDefaultName,
  catalogPreviewURL,
};
