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

  document.addEventListener("htmx:afterRequest", function (event) {
    if (!event.detail.successful) return;

    // Close the <details> accordion after a lesson review is submitted.
    // The list refresh is handled by the HX-Trigger response header.
    var elt = event.detail.elt || event.target;
    if (elt && elt.classList && elt.classList.contains("lesson-review-form")) {
      var details = elt.closest("details");
      if (details) details.open = false;
      elt.reset();
      return;
    }

    if (event.target.closest && event.target.closest("form")) {
      document.body.dispatchEvent(new CustomEvent("refreshLessons"));
    }
  });
});
