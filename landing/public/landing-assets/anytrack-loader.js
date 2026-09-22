// This public property loader mirrors AnyTrack's generated tag. The property
// ID is intentionally public; credentials and Meta tokens never enter it.
!(function (scope, document, tag, name, script) {
  script = document.createElement(tag);
  script.async = true;
  script.src = "https://assets.anytrack.io/BZgLiRKLRvlt.js";
  document.getElementsByTagName(tag)[0].parentNode.insertBefore(script, null);
  scope[name] =
    scope[name] ||
    function () {
      (scope[name].q = scope[name].q || []).push(arguments);
    };
})(window, document, "script", "AnyTrack");
