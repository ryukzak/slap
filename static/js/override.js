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

document.addEventListener("click", function (e) {
  var btn = e.target.closest("[data-action]");
  if (!btn) return;

  var action = btn.getAttribute("data-action");

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

// Initialize all accordions when the DOM is loaded
document.addEventListener("DOMContentLoaded", function () {
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

  // Save open state before htmx swaps lesson records.
  var openRecords = {};
  document.addEventListener("htmx:beforeSwap", function (event) {
    if (event.detail.target.id !== "lesson-task-records") return;
    openRecords = {};
    event.detail.target.querySelectorAll("details[data-record-id]").forEach(function (d) {
      if (d.open) openRecords[d.getAttribute("data-record-id")] = true;
    });
    // The just-submitted record should be closed after refresh.
    if (submittedRecordId) {
      delete openRecords[submittedRecordId];
      submittedRecordId = null;
    }
  });

  // Restore open state after htmx swaps lesson records.
  document.addEventListener("htmx:afterSwap", function (event) {
    if (event.detail.target.id !== "lesson-task-records") return;
    event.detail.target.querySelectorAll("details[data-record-id]").forEach(function (d) {
      if (openRecords[d.getAttribute("data-record-id")]) d.open = true;
    });
    openRecords = {};
  });
});
