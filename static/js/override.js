function toggleAccordion(header) {
  const content = header.nextElementSibling;
  if (!content || !content.classList.contains("accordion-content")) {
    return;
  }

  const icon = header.querySelector(".accordion-icon");

  if (content.style.display === "none") {
    content.style.display = "block";
    if (icon) icon.textContent = "-";
  } else {
    content.style.display = "none";
    if (icon) icon.textContent = "+";
  }
}

function normalizeTheme(theme) {
  if (theme === "light" || theme === "dark") return theme;
  return null;
}

function storedTheme() {
  try {
    return normalizeTheme(localStorage.getItem("slap-theme"));
  } catch (e) {
    return null;
  }
}

function preferredTheme() {
  var stored = storedTheme();
  if (stored) return stored;
  if (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches) {
    return "dark";
  }
  return "light";
}

function currentTheme() {
  return normalizeTheme(document.documentElement.getAttribute("data-theme")) || preferredTheme();
}

function storeTheme(theme) {
  try {
    localStorage.setItem("slap-theme", theme);
  } catch (e) {}
}

function applyTheme(theme) {
  theme = normalizeTheme(theme) || preferredTheme();
  document.documentElement.setAttribute("data-theme", theme);
}

function setTheme(theme) {
  theme = normalizeTheme(theme) || preferredTheme();
  applyTheme(theme);
  storeTheme(theme);
}

document.addEventListener("click", function (e) {
  var btn = e.target.closest("[data-action]");
  if (!btn) return;

  var action = btn.getAttribute("data-action");

  if (action === "toggle-theme") {
    setTheme(currentTheme() === "dark" ? "light" : "dark");
  }

  if (action === "delete-lesson") {
    if (!confirm("Delete this lesson?")) return;
    fetch(btn.getAttribute("data-url"), { method: "DELETE" }).then(function (r) {
      if (r.ok) window.location.href = btn.getAttribute("data-redirect");
      else alert("Failed to delete");
    });
  }

  if (action === "revoke-all") {
    if (!confirm("Revoke all queued registrations?")) return;
    document.getElementById(btn.getAttribute("data-form")).submit();
  }

  if (action === "open-all") {
    var d = document.querySelectorAll(btn.getAttribute("data-target"));
    var s = ![].slice.call(d).every(function (x) { return x.open; });
    d.forEach(function (x) { x.open = s; });
  }
});

// Recently used lesson descriptions, newest first, kept in localStorage so
// the "add lesson" form can offer them as presets.
var RECENT_LESSON_DESCRIPTIONS_KEY = "slap-lesson-descriptions";
var RECENT_LESSON_DESCRIPTIONS_MAX = 5;

function loadRecentLessonDescriptions() {
  try {
    var list = JSON.parse(localStorage.getItem(RECENT_LESSON_DESCRIPTIONS_KEY));
    if (Array.isArray(list)) return list.filter(function (d) { return typeof d === "string" && d.trim(); });
    // migrate the single description remembered by earlier versions
    var last = localStorage.getItem("slap-lesson-description");
    return last && last.trim() ? [last] : [];
  } catch (e) {
    return [];
  }
}

function saveRecentLessonDescription(description) {
  if (!description.trim()) return;
  var list = loadRecentLessonDescriptions().filter(function (d) { return d !== description; });
  list.unshift(description);
  try {
    localStorage.setItem(RECENT_LESSON_DESCRIPTIONS_KEY,
      JSON.stringify(list.slice(0, RECENT_LESSON_DESCRIPTIONS_MAX)));
    localStorage.removeItem("slap-lesson-description");
  } catch (e) {}
}

function lessonPresetLabel(description) {
  var line = description.trim().split("\n")[0].replace(/^#+\s*/, "").trim();
  return line.length > 40 ? line.slice(0, 39) + "…" : line;
}

function renderLessonPresets(textarea, recent, configDefault) {
  var container = document.getElementById("lesson-presets");
  if (!container || !recent.length) return;
  function addPreset(label, description) {
    var button = document.createElement("button");
    button.type = "button";
    button.className = "text-blue-400 hover:text-blue-300 text-xs";
    button.textContent = "[" + label + "]";
    button.title = description;
    button.addEventListener("click", function () {
      textarea.value = description;
      textarea.focus();
    });
    container.appendChild(button);
  }
  recent.forEach(function (description) {
    addPreset(lessonPresetLabel(description), description);
  });
  if (configDefault.trim() && recent.indexOf(configDefault) === -1) {
    addPreset("default", configDefault);
  }
  container.classList.remove("hidden");
}

// Initialize all accordions when the DOM is loaded
document.addEventListener("DOMContentLoaded", function () {
  applyTheme(currentTheme());

  const accordionHeaders = document.querySelectorAll(".accordion-header");

  accordionHeaders.forEach((header) => {
    const content = header.nextElementSibling;
    if (content && content.classList.contains("accordion-content")) {
      header.addEventListener("click", function (event) {
        event.preventDefault();
        toggleAccordion(this);
      });
    }
  });

  // Track which review was just submitted so we close it after refresh.
  var submittedRecordId = null;

  document.addEventListener("htmx:afterRequest", function (event) {
    if (!event.detail.successful) return;

    var elt = event.detail.elt || event.target;
    if (elt && elt.classList && elt.classList.contains("lesson-review-form")) {
      var details = elt.closest("details");
      if (details) submittedRecordId = details.getAttribute("data-record-id");
      elt.reset();
      return;
    }

    if (event.target.closest && event.target.closest("form")) {
      document.body.dispatchEvent(new CustomEvent("refreshLessons"));
    }
  });

  // Save open state and textarea content before htmx swaps lesson records.
  var openRecords = {};
  var savedTextareas = {};
  document.addEventListener("htmx:beforeSwap", function (event) {
    if (event.detail.target.id !== "lesson-task-records") return;
    openRecords = {};
    savedTextareas = {};
    event.detail.target.querySelectorAll("details[data-record-id]").forEach(function (d) {
      var id = d.getAttribute("data-record-id");
      if (d.open) openRecords[id] = true;
      var ta = d.querySelector("textarea");
      if (ta && ta.value) savedTextareas[id] = ta.value;
    });
    // The just-submitted record should be closed after refresh; don't restore its textarea.
    if (submittedRecordId) {
      delete openRecords[submittedRecordId];
      delete savedTextareas[submittedRecordId];
      submittedRecordId = null;
    }
  });

  // Restore open state and textarea content after htmx swaps lesson records.
  document.addEventListener("htmx:afterSwap", function (event) {
    if (event.detail.target.id !== "lesson-task-records") return;
    event.detail.target.querySelectorAll("details[data-record-id]").forEach(function (d) {
      var id = d.getAttribute("data-record-id");
      if (openRecords[id]) d.open = true;
      var ta = d.querySelector("textarea");
      if (ta && savedTextareas[id]) ta.value = savedTextareas[id];
    });
    openRecords = {};
    savedTextareas = {};
  });

  // Remember the last few descriptions used to create a lesson: prefill the
  // "add lesson" form with the latest one (overriding the configured default)
  // and offer all of them as presets.
  var lessonDescription = document.getElementById("lesson-description");
  if (lessonDescription) {
    var configDefault = lessonDescription.defaultValue;
    var recent = loadRecentLessonDescriptions();
    if (recent.length) lessonDescription.value = recent[0];
    renderLessonPresets(lessonDescription, recent, configDefault);
    if (lessonDescription.form) {
      lessonDescription.form.addEventListener("submit", function () {
        saveRecentLessonDescription(lessonDescription.value);
      });
    }
  }
});
