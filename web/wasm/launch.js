const splash = document.getElementById("splash");
const shell = document.getElementById("game-shell");
const startButton = document.getElementById("start-button");
const fullscreenButton = document.getElementById("fullscreen-button");
const buildPill = document.getElementById("build-pill");

let splashDismissed = false;

function getBuildID() {
  return typeof window.__gdwolfBuildID === "string" ? window.__gdwolfBuildID : "";
}

function isMobileLike() {
  if (typeof navigator === "undefined") {
    return false;
  }
  return /Android|iPad|iPhone|iPod/i.test(navigator.userAgent) || (
    navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1
  );
}

function getPlayerURL() {
  const url = new URL("./player.html", window.location.href);
  const buildID = getBuildID();
  if (buildID) {
    url.searchParams.set("v", buildID);
  }
  return url.toString();
}

function isInteractiveTarget(target) {
  if (!(target instanceof Element)) {
    return false;
  }
  return Boolean(target.closest("a, button, input, select, textarea, summary, [role='button'], [tabindex]"));
}

function hideSplash() {
  if (splashDismissed || !splash) {
    return;
  }
  splashDismissed = true;
  splash.hidden = true;
}

function focusPlayer() {
  if (!shell) {
    return;
  }
  try {
    shell.focus({ preventScroll: true });
  } catch (_err) {
  }
  if (!shell.contentWindow) {
    return;
  }
  try {
    shell.contentWindow.focus();
    shell.contentWindow.postMessage({ type: "gdwolf-claim-focus" }, window.location.origin);
  } catch (_err) {
  }
}

async function requestFullscreen() {
  if (!shell) {
    return;
  }
  const target = shell;
  const request = target.requestFullscreen || target.webkitRequestFullscreen;
  if (typeof request !== "function") {
    return;
  }
  try {
    await request.call(target);
    focusPlayer();
  } catch (_err) {
  }
}

function claimFocusAndStart() {
  hideSplash();
  focusPlayer();
}

function initializePlayerFrame() {
  if (!shell) {
    return;
  }
  const target = getPlayerURL();
  if (shell.src !== target) {
    shell.src = target;
  }
}

function updateBuildPill() {
  if (!buildPill) {
    return;
  }
  const buildID = getBuildID();
  buildPill.textContent = buildID ? `Build: ${buildID}` : "Build: local";
}

updateBuildPill();
initializePlayerFrame();

if (isMobileLike()) {
  window.location.replace(getPlayerURL());
}

if (splash) {
  splash.addEventListener("click", (event) => {
    if (isInteractiveTarget(event.target)) {
      return;
    }
    event.preventDefault();
    claimFocusAndStart();
  });
  splash.addEventListener("touchstart", (event) => {
    if (isInteractiveTarget(event.target)) {
      return;
    }
    event.preventDefault();
    claimFocusAndStart();
  }, { passive: false });
}

if (startButton) {
  startButton.addEventListener("click", () => {
    claimFocusAndStart();
  });
}

if (fullscreenButton) {
  fullscreenButton.addEventListener("click", () => {
    requestFullscreen();
  });
}

window.addEventListener("keydown", (event) => {
  if (isInteractiveTarget(event.target)) {
    return;
  }
  if (event.key !== "Enter" && event.key !== " " && event.key !== "Spacebar") {
    return;
  }
  if (splashDismissed) {
    return;
  }
  event.preventDefault();
  claimFocusAndStart();
});

window.addEventListener("message", (event) => {
  if (event.origin !== window.location.origin || !event.data) {
    return;
  }
  if (event.data.type !== "gdwolf-player-ready") {
    return;
  }
  window.requestAnimationFrame(() => {
    focusPlayer();
  });
});

document.addEventListener("fullscreenchange", () => {
  if (document.fullscreenElement) {
    window.requestAnimationFrame(() => {
      focusPlayer();
    });
    return;
  }
  if (splashDismissed) {
    window.requestAnimationFrame(() => {
      focusPlayer();
    });
  }
});
