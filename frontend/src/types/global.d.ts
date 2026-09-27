export {};

declare global {
  interface Window {
    PackageR: any;
    grecaptcha: any;
    PasswordCredential?: {
      new (form: HTMLFormElement): Credential;
    };
  }

  interface HTMLElement {
    // TODO: no idea what the exact type is
    __vue__: any;
  }
}
