"use strict";

// Who can get in and who this isoshelf talks to: the username and password,
// signing out, the other isoshelf to copy from, and sharing with others.
// Each has a request of its own rather than going through /api/settings.

// loginControl is the username and password: a button that opens the form,
// and the two ways out. Changing it is rare enough that it does not need to
// sit open in the panel taking up room.
function loginControl() {
  const login = (state && state.login) || {};
  const buttons = [
    el("button", { type: "button", class: "btn small", onclick: changeLogin },
      login.user ? "Change username or password" : "Set a username and password"),
  ];
  if (login.user) {
    buttons.push(
      el("a", { class: "btn small", href: "/login", onclick: signOut }, "Sign out"),
      el("button", { type: "button", class: "btn small", onclick: signOutEverywhere },
        "Sign out everywhere"));
  }
  return el("div", { class: "setting-controls" }, buttons);
}

// changeLogin asks for the new username and password, and the old one unless
// this browser got in with the link - in which case the old password is the
// thing that has been forgotten.
async function changeLogin() {
  const login = (state && state.login) || {};
  const body = el("div", { class: "report" });
  const field = (label, name, type, hint) => {
    const input = el("input", { type, id: `login-${name}`, class: "records-dir",
      autocomplete: type === "password" ? "new-password" : "username" });
    body.append(el("div", {},
      el("label", { for: `login-${name}` }, label),
      input,
      hint ? el("div", { class: "muted setting-note" }, hint) : null));
    return input;
  };

  let current = null;
  if (login.user) {
    current = field("Your password now", "current", "password");
    current.autocomplete = "current-password";
  }
  const user = field("Username", "user", "text");
  user.value = login.user || "";
  const password = field("Password", "password", "password");
  const again = field("Password again", "again", "password");
  body.append(el("p", { class: "muted" },
    "Every other browser is signed out when this changes, so a changed password " +
    "means a changed password everywhere."));

  const go = await ask(login.user ? "Change the login" : "Set a username and password", "", [
    { label: "Save", value: "go", primary: true },
    { label: "Cancel", value: "" },
  ], body);
  if (!go) return;
  try {
    state = await api("POST", "/api/login", {
      user: user.value,
      current: current ? current.value : "",
      password: password.value,
      again: again.value,
    });
    render();
    showNotice("Saved. Everywhere else has been signed out.", false);
  } catch (err) {
    showNotice(err.message, true);
  }
}

// signOut ends this browser's session, then goes to the login page.
async function signOut(e) {
  e.preventDefault();
  try {
    await api("POST", "/api/login/signout", {});
  } catch (err) {
    showNotice(err.message, true);
    return;
  }
  location.href = "/login";
}

async function signOutEverywhere() {
  const yes = await ask("Sign out everywhere?",
    "Every browser signed in to this isoshelf, including this one, has to sign in " +
    "again. The username and password don't change.",
    [{ label: "Sign out everywhere", value: "yes", primary: true }, { label: "Cancel", value: null }]);
  if (!yes) return;
  try {
    await api("POST", "/api/login/everywhere", {});
  } catch {
    // Signing out and then being refused is the point; either way, reload.
  }
  location.href = "/login";
}

// peerControl is the other isoshelf: its address, a way to change it, and a
// switch to stop using it without forgetting where it was.
function peerControl() {
  const p = (state && state.peer) || {};
  if (!p.address) {
    return el("div", { class: "setting-controls" },
      el("button", { type: "button", class: "btn small", onclick: choosePeer }, "Add an isoshelf"));
  }
  return el("div", { class: "setting-controls" },
    switchRow("Use it", p.on, null, (on) => savePeer({ off: !on })),
    el("button", { type: "button", class: "btn small", onclick: choosePeer }, "Change"),
    el("button", { type: "button", class: "btn small", onclick: () => savePeer({ forget: true }) }, "Forget it"));
}

function peerNote() {
  const p = (state && state.peer) || {};
  if (!p.address) return "";
  const who = p.user ? `${p.address}, signed in as ${p.user}` : p.address;
  return p.on
    ? `${who}. Anything it doesn't have still comes from the internet, and every file is ` +
      "checked against the project's own checksum either way."
    : `${who} - not being used at the moment.`;
}

// choosePeer asks for the address and the login for it.
async function choosePeer() {
  const p = (state && state.peer) || {};
  const body = el("div", { class: "report" });
  const field = (label, name, type, value, hint) => {
    const input = el("input", { type, id: `peer-${name}`, class: "records-dir", value: value || "" });
    body.append(el("div", {},
      el("label", { for: `peer-${name}` }, label),
      input,
      hint ? el("div", { class: "muted setting-note" }, hint) : null));
    return input;
  };
  const address = field("Address", "address", "text", p.address, "Like 10.0.0.5:8765.");
  const user = field("Username there", "user", "text", p.user);
  const password = field("Password there", "password", "password", "",
    p.address ? "Leave empty to keep the one already saved." : "");
  body.append(el("p", { class: "muted" },
    "The password is kept on this computer in isoshelf's own folder, and travels over " +
    "your network unencrypted - the same as opening that isoshelf in a browser. " +
    "Sharing has to be turned on over there as well."));

  const go = await ask("Another isoshelf on your network", "", [
    { label: "Save", value: "go", primary: true },
    { label: "Cancel", value: "" },
  ], body);
  if (!go) return;
  await savePeer({ address: address.value, user: user.value, password: password.value });
}

async function savePeer(change) {
  try {
    state = await api("POST", "/api/peer", change);
    render();
    showNotice(change.forget ? "Forgotten." : "Saved.", false);
  } catch (err) {
    showNotice(err.message, true);
  }
}

// shareImages turns this isoshelf's own sharing on or off. It has its own
// request rather than going through /api/settings, because turning it on
// changes what a scan does.
async function shareImages(on) {
  try {
    state = await api("POST", "/api/share", { share: on });
    saved(["share"]);
    render();
  } catch (err) {
    showNotice(err.message, true);
  }
}
