const $ = (selector) => document.querySelector(selector);
const state = {
  exercises: [],
  attempts: [],
  active: null,
  inspectedFile: null,
  filter: "all",
  query: "",
};
const learningOrder = [
  "merge-basic",
  "add-add",
  "modify-delete",
  "rebase-basic",
  "merge-multi",
  "cherry-pick",
];
const branches = {
  "merge-basic": "feature/login",
  "add-add": "feature/notes",
  "modify-delete": "feature/cleanup",
  "merge-multi": "feature/release",
};
const iconNames = {
  "merge-basic": "branch",
  "add-add": "grid",
  "modify-delete": "history",
  "rebase-basic": "terminal",
  "merge-multi": "grid",
  "cherry-pick": "branch",
};
const categories = {
  "merge-basic": "merge",
  "add-add": "add/add",
  "modify-delete": "modify/delete",
  "rebase-basic": "rebase",
  "merge-multi": "multi-file",
  "cherry-pick": "cherry-pick",
};
const badges = [
  {
    id: "first",
    name: "First Resolution",
    description: "Complete your first challenge.",
    earned: (done) => done.size >= 1,
  },
  {
    id: "merge",
    name: "Merge Maker",
    description: "Resolve the basic merge conflict.",
    earned: (done) => done.has("merge-basic"),
  },
  {
    id: "two",
    name: "On a Roll",
    description: "Complete two different scenarios.",
    earned: (done) => done.size >= 2,
  },
  {
    id: "rebase",
    name: "History Rewriter",
    description: "Finish the rebase challenge.",
    earned: (done) => done.has("rebase-basic"),
  },
  {
    id: "multi",
    name: "Two Files, One Merge",
    description: "Resolve the multi-file merge.",
    earned: (done) => done.has("merge-multi"),
  },
  {
    id: "pick",
    name: "Selective Integrator",
    description: "Finish the cherry-pick challenge.",
    earned: (done) => done.has("cherry-pick"),
  },
  {
    id: "all",
    name: "Conflict Pro",
    description: "Complete every scenario.",
    earned: (done, total) => total > 0 && done.size === total,
  },
];

const skillDefinitions = {
  "read-conflicts": [
    "Read conflicts",
    "Identify each branch's change from the conflict markers.",
  ],
  "merge-history": [
    "Complete a merge",
    "Finish a merge commit with both original parents.",
  ],
  "combine-content": [
    "Combine content",
    "Preserve intended changes from both branches.",
  ],
  "deletion-resolution": [
    "Resolve deletions",
    "Decide whether a removed file should remain deleted.",
  ],
  "rebase-history": ["Rebase history", "Replay a commit on top of main."],
  "multi-file-resolution": [
    "Resolve multiple files",
    "Clear every unmerged path before committing.",
  ],
  "cherry-pick-history": [
    "Cherry-pick a commit",
    "Replay one selected commit onto main.",
  ],
};

function icon(name, className = "icon") {
  return `<svg class="${className}" aria-hidden="true"><use href="#i-${name}"></use></svg>`;
}
function escapeHTML(value) {
  return String(value).replace(
    /[&<>"']/g,
    (character) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        character
      ],
  );
}
async function request(url, data) {
  const options = data
    ? {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      }
    : {};
  const response = await fetch(url, options);
  if (!response.ok)
    throw new Error(
      (await response.text()).trim() || `Request failed (${response.status})`,
    );
  return response.json();
}

function completedIDs() {
  return new Set(
    state.attempts
      .filter((attempt) => attempt.status === "Completed")
      .map((attempt) => attempt.id),
  );
}
function unlockedIDs(done = completedIDs()) {
  return new Set(
    badges
      .filter((badge) => badge.earned(done, state.exercises.length))
      .map((badge) => badge.id),
  );
}

function renderExercises() {
  const filtered = state.exercises.filter((exercise) => {
    const matchesDifficulty =
      state.filter === "all" || state.filter === exercise.difficulty;
    const text =
      `${exercise.title} ${exercise.objective} ${categories[exercise.id] || ""}`.toLowerCase();
    return matchesDifficulty && text.includes(state.query);
  });
  $("#nav-count").textContent = String(state.exercises.length).padStart(2, "0");
  $("#exercise-count").textContent =
    `${filtered.length} ${filtered.length === 1 ? "EXERCISE" : "EXERCISES"}`;
  $("#exercise-grid").setAttribute("aria-busy", "false");
  $("#no-results").hidden = filtered.length > 0;
  $("#exercise-grid").innerHTML = filtered
    .map((exercise) => {
      const sequence = state.exercises.indexOf(exercise) + 1;
      const done = completedIDs().has(exercise.id);
      return `<article class="challenge-row">
      <span class="challenge-icon">${icon(iconNames[exercise.id] || "grid")}</span>
      <div class="challenge-copy"><h3>${escapeHTML(exercise.title)}</h3><p>${escapeHTML(exercise.objective)}</p>
      <div class="challenge-tags"><span class="tag ${escapeHTML(exercise.difficulty)}">${escapeHTML(exercise.difficulty)}</span><span class="tag">${escapeHTML(categories[exercise.id] || "Git")}</span>${done ? '<span class="tag easy">completed</span>' : ""}</div></div>
      <button class="challenge-action" data-start="${escapeHTML(exercise.id)}" aria-label="Start ${escapeHTML(exercise.title)}">${done ? "Practice again" : `Start #${String(sequence).padStart(2, "0")}`} ${icon("arrow")}</button>
    </article>`;
    })
    .join("");
  document
    .querySelectorAll("[data-start]")
    .forEach((button) =>
      button.addEventListener("click", () =>
        start(button.dataset.start, button),
      ),
    );
}

function renderAttempts() {
  const list = $("#attempt-list");
  list.setAttribute("aria-busy", "false");
  if (!state.attempts.length) {
    list.innerHTML =
      '<div class="empty-attempts">No attempts yet. Choose a challenge above to create your first lab.</div>';
    return;
  }
  list.innerHTML = state.attempts
    .map((attempt) => {
      const exercise = state.exercises.find((item) => item.id === attempt.id);
      return `<div class="attempt-row"><span class="attempt-mark">${icon("history")}</span><div class="attempt-info"><strong>${escapeHTML(exercise?.title || attempt.id)}</strong><small>${escapeHTML(attempt.path)}</small></div><span class="attempt-status ${attempt.status === "Completed" ? "complete" : ""}">${escapeHTML(attempt.status || "Not started")}</span><button data-open="${escapeHTML(attempt.folder)}">Open →</button></div>`;
    })
    .join("");
  document.querySelectorAll("[data-open]").forEach((button) =>
    button.addEventListener("click", () => {
      const attempt = state.attempts.find(
        (item) => item.folder === button.dataset.open,
      );
      if (attempt) activate(attempt);
    }),
  );
}

function renderAchievements() {
  const done = completedIDs();
  const unlocked = unlockedIDs(done);
  const total = state.exercises.length;
  $("#completed-count").textContent = String(done.size);
  $(".progress-number span").textContent = `/ ${total}`;
  $("#rail-progress-text").textContent = `${done.size} of ${total} completed`;
  const percent = total ? (done.size / total) * 100 : 0;
  $("#progress-bar").style.width = `${percent}%`;
  $("#rail-progress-bar").style.width = `${percent}%`;
  $("#achievement-count").textContent =
    `${unlocked.size} / ${badges.length} UNLOCKED`;
  $("#achievement-grid").innerHTML = badges
    .map((badge) => {
      const earned = unlocked.has(badge.id);
      const medalIcon =
        badge.id === "all" || badge.id === "first"
          ? "trophy"
          : badge.id === "rebase"
            ? "history"
            : badge.id === "merge"
              ? "branch"
              : "spark";
      return `<div class="achievement-card ${earned ? "unlocked" : ""}"><span class="achievement-medal">${icon(medalIcon)}</span><span><strong>${escapeHTML(badge.name)}</strong><small>${escapeHTML(badge.description)}</small></span><span class="achievement-lock">${earned ? "UNLOCKED" : "LOCKED"}</span></div>`;
    })
    .join("");
  const next = badges.find((badge) => !unlocked.has(badge.id));
  $(".earn-card h3").textContent = next
    ? `Next: ${next.name}`
    : "All badges unlocked.";
  $(".earn-card p").textContent = next
    ? next.description
    : "You've completed every challenge in the lab.";
}

function renderSkills() {
  const done = completedIDs();
  let earned = 0;
  $("#skill-grid").innerHTML = Object.entries(skillDefinitions)
    .map(([id, [name, description]]) => {
      const related = state.exercises.filter((exercise) =>
        exercise.skills.includes(id),
      );
      const completed = related.filter((exercise) => done.has(exercise.id));
      if (completed.length) earned++;
      return `<article class="skill-card ${completed.length ? "earned" : ""}"><span class="skill-icon">${icon(completed.length ? "check" : "branch")}</span><div><strong>${escapeHTML(name)}</strong><p>${escapeHTML(description)}</p><small>${completed.length ? `Proven in ${completed.length} ${completed.length === 1 ? "exercise" : "exercises"}` : `Practice in ${escapeHTML(related[0]?.title || "an exercise")}`}</small></div></article>`;
    })
    .join("");
  $("#skill-count").textContent =
    `${earned} / ${Object.keys(skillDefinitions).length} SKILLS`;
}

function showLoading(message) {
  $("#screen-loading-text").textContent = message;
  $("#screen-loading").hidden = false;
}
function hideLoading() {
  $("#screen-loading").hidden = true;
}
function showToast(message) {
  const toast = $("#toast");
  toast.textContent = message;
  toast.hidden = false;
  clearTimeout(showToast.timer);
  showToast.timer = setTimeout(() => {
    toast.hidden = true;
  }, 4500);
}

async function start(id, button) {
  button.disabled = true;
  showLoading("Building your practice repository…");
  try {
    let attemptName = "";
    if (state.attempts.some((item) => item.folder === id)) {
      let number = 2;
      while (
        state.attempts.some((item) => item.folder === `${id}-attempt-${number}`)
      )
        number++;
      attemptName = `attempt-${number}`;
    }
    const created = await request("/api/start", { id, attempt: attemptName });
    state.attempts.push(created);
    renderAttempts();
    renderAchievements();
    renderSkills();
    activate(created);
  } catch (error) {
    showToast(`Could not create the exercise: ${error.message}`);
  } finally {
    hideLoading();
    button.disabled = false;
  }
}

function activate(attempt) {
  state.active = attempt;
  state.inspectedFile = null;
  const exercise = state.exercises.find((item) => item.id === attempt.id);
  if (!exercise) return;
  $("#workspace").hidden = false;
  $("#workspace-id").textContent = attempt.folder.toUpperCase();
  $("#active-title").textContent = exercise.title;
  $("#active-description").textContent = exercise.description;
  renderConcept(exercise);
  $("#active-path").textContent = attempt.path;
  $("#command-cd").textContent = `cd "${attempt.path}"`;
  $("#command-merge").textContent =
    attempt.id === "rebase-basic"
      ? "git switch feature/fast\ngit rebase main"
      : attempt.id === "cherry-pick"
        ? "git cherry-pick feature/audit"
        : `git merge ${branches[attempt.id]}`;
  $("#resolution-step").textContent =
    attempt.id === "rebase-basic"
      ? "Edit settings.txt, stage it, then run git rebase --continue."
      : attempt.id === "cherry-pick"
        ? "Set policy.txt to audit=required+verbose, stage it, then run git cherry-pick --continue."
        : attempt.id === "merge-multi"
          ? "Resolve config.txt and review.txt, stage both, then commit."
          : attempt.id === "modify-delete"
            ? "Use git rm legacy.txt, then run git commit."
            : "Edit the conflicted file, then complete the merge in your terminal.";
  $("#workspace-state").textContent = (
    attempt.status || "Not started"
  ).toUpperCase();
  $("#workspace-state").classList.toggle(
    "complete",
    attempt.status === "Completed",
  );
  $("#feedback-text").textContent = "Your next step will appear here.";
  $("#feedback").className = "feedback";
  $("#hint-list").replaceChildren();
  inspect(attempt);
  $("#workspace").scrollIntoView({ behavior: "smooth", block: "start" });
}

function renderConcept(exercise) {
  const lesson = exercise.lesson;
  $("#concept-principle").textContent = lesson.principle;
  $("#concept-question").textContent = lesson.question;
  $("#concept-feedback").hidden = true;
  $("#concept-feedback").textContent = "";
  $("#concept-feedback").className = "concept-feedback";
  $("#concept-panel .pitfalls").open = false;
  $("#concept-pitfalls").innerHTML = lesson.pitfalls
    .map((item) => `<li>${escapeHTML(item)}</li>`)
    .join("");
  $("#concept-options").innerHTML = lesson.options
    .map(
      (option, index) =>
        `<button type="button" class="concept-option" data-choice="${index}" aria-pressed="false"><span>${String.fromCharCode(65 + index)}</span>${escapeHTML(option)}</button>`,
    )
    .join("");
  $("#concept-options")
    .querySelectorAll("[data-choice]")
    .forEach((button) =>
      button.addEventListener("click", () => {
        const correct = Number(button.dataset.choice) === lesson.correct;
        $("#concept-options")
          .querySelectorAll("[data-choice]")
          .forEach((choice) => {
            choice.classList.toggle("selected", choice === button);
            choice.setAttribute("aria-pressed", String(choice === button));
          });
        const feedback = $("#concept-feedback");
        feedback.hidden = false;
        feedback.className = `concept-feedback ${correct ? "correct" : "incorrect"}`;
        feedback.textContent = `${correct ? "That's right." : "Not quite. Try another answer."} ${lesson.explanation}`;
      }),
    );
}

const lessonNotes = {
  "merge-basic":
    "Main tightened password length. The feature branch also checks user name length. Your merge should preserve both requirements.",
  "add-add":
    "Neither branch inherited this file. Git found two different new files at the same path, so the final file needs both notes.",
  "modify-delete":
    "The file existed in the common ancestor. Main edited it, while the feature branch removed it. This exercise asks you to keep the deliberate deletion.",
  "rebase-basic":
    "Both branches changed the same setting. During rebase, the feature commit is replayed after main, and the final mode should combine safety with speed.",
  "merge-multi":
    "Main and feature changed two files differently. Inspect each file, then keep the longer timeout and both review methods.",
  "cherry-pick":
    "Replay only the audit feature commit onto main, keeping both the required and verbose audit policies.",
};

async function inspect(attempt = state.active, selected = state.inspectedFile) {
  if (!attempt) return;
  const grid = $("#version-grid");
  grid.setAttribute("aria-busy", "true");
  try {
    const view = await request("/api/inspect", {
      folder: attempt.folder,
      file: selected || "",
    });
    if (state.active?.folder !== attempt.folder) return;
    state.inspectedFile = view.file;
    $("#lesson-explanation").textContent = lessonNotes[attempt.id];
    $("#file-tabs").innerHTML =
      view.files.length > 1
        ? view.files
            .map(
              (file) =>
                `<button type="button" class="file-tab ${file === view.file ? "active" : ""}" data-file="${escapeHTML(file)}" aria-pressed="${file === view.file}">${escapeHTML(file)}</button>`,
            )
            .join("")
        : "";
    $("#file-tabs")
      .querySelectorAll("[data-file]")
      .forEach((button) =>
        button.addEventListener("click", () =>
          inspect(attempt, button.dataset.file),
        ),
      );
    const versions = [
      ["Common ancestor", view.base, "Before the branches diverged"],
      ["Main branch", view.main, "The original main commit"],
      [view.branch, view.feature, "The original feature commit"],
      ["Your working file", view.current, "Updates when you refresh"],
    ];
    grid.innerHTML = versions
      .map(
        ([label, version, caption]) =>
          `<div class="version-card"><div class="version-head"><strong>${escapeHTML(label)}</strong><small>${escapeHTML(caption)}</small></div><pre>${version.exists ? escapeHTML(version.content) : '<span class="file-absent">File does not exist in this version</span>'}</pre></div>`,
      )
      .join("");
    $("#review-panel").innerHTML =
      `<strong>${view.check.Passed ? "Verified resolution" : "Next step"}</strong><p>${escapeHTML(view.check.Message)}</p><small>Git Conflict Lab checks the actual Git history and the final file, not just the text shown here.</small>`;
  } catch (error) {
    if (state.active?.folder === attempt.folder)
      $("#review-panel").textContent =
        `Could not inspect this repository: ${error.message}`;
  } finally {
    grid.setAttribute("aria-busy", "false");
  }
}

function showSuccess(message, newBadges) {
  $("#success-message").textContent = message;
  $("#unlocked-badges").innerHTML = newBadges
    .map(
      (badge) =>
        `<div class="unlocked-badge">✦ New achievement: ${escapeHTML(badge.name)}</div>`,
    )
    .join("");
  const dialog = $("#success-dialog");
  if (!dialog.open) dialog.showModal();
}

async function check() {
  if (!state.active) return;
  const button = $("#check-button");
  button.disabled = true;
  button.querySelector("span").textContent = "Checking Git history…";
  const wasComplete = state.active.status === "Completed";
  const oldBadges = unlockedIDs();
  try {
    const result = await request("/api/check", { folder: state.active.folder });
    $("#feedback-text").textContent = result.Message;
    $("#feedback").className =
      `feedback ${result.Passed ? "success" : "error"}`;
    state.active.status = result.Passed
      ? "Completed"
      : result.Message.includes("not been merged yet") ||
          result.Message.includes("has not been applied yet") ||
          result.Message.startsWith("Switch to feature/fast")
        ? "Not started"
        : "In progress";
    $("#workspace-state").textContent = state.active.status.toUpperCase();
    $("#workspace-state").classList.toggle("complete", result.Passed);
    renderAttempts();
    renderAchievements();
    renderSkills();
    renderExercises();
    await inspect();
    if (result.Passed && !wasComplete) {
      const newlyEarned = badges.filter(
        (badge) => unlockedIDs().has(badge.id) && !oldBadges.has(badge.id),
      );
      showSuccess(result.Message, newlyEarned);
    }
  } catch (error) {
    $("#feedback-text").textContent = error.message;
    $("#feedback").className = "feedback error";
  } finally {
    button.disabled = false;
    button.querySelector("span").textContent = "Check my solution";
  }
}

async function hint() {
  if (!state.active) return;
  const button = $("#hint-button");
  button.disabled = true;
  button.querySelector("span").textContent = "Finding the next hint…";
  try {
    const result = await request("/api/hint", { folder: state.active.folder });
    if (
      [...$("#hint-list").children].some(
        (node) => node.textContent === result.hint,
      )
    )
      return;
    const entry = document.createElement("div");
    entry.className = "hint-entry";
    entry.textContent = result.hint;
    $("#hint-list").append(entry);
  } catch (error) {
    showToast(error.message);
  } finally {
    button.disabled = false;
    button.querySelector("span").textContent = "Reveal a hint";
  }
}

async function copy(value, button) {
  try {
    await navigator.clipboard.writeText(value);
    const original = button.getAttribute("aria-label") || button.textContent;
    if (button.id === "copy-path")
      button.setAttribute("aria-label", "Copied path");
    else button.textContent = "Copied!";
    setTimeout(() => {
      if (button.id === "copy-path")
        button.setAttribute("aria-label", original);
      else button.textContent = original;
    }, 1500);
  } catch {
    showToast(
      "Clipboard access is unavailable. Select and copy the command above.",
    );
  }
}

async function init() {
  try {
    [state.exercises, state.attempts] = await Promise.all([
      request("/api/exercises"),
      request("/api/attempts"),
    ]);
    const rank = (id) => {
      const index = learningOrder.indexOf(id);
      return index < 0 ? 999 : index;
    };
    state.exercises.sort((a, b) => rank(a.id) - rank(b.id));
    renderExercises();
    renderAttempts();
    renderAchievements();
    renderSkills();
  } catch (error) {
    $("#exercise-grid").textContent =
      `Could not load the lab: ${error.message}`;
    $("#attempt-list").textContent = "Attempts are unavailable.";
    showToast("Could not load the lab. Restart the local server and refresh.");
  }
  $("#check-button").addEventListener("click", check);
  $("#hint-button").addEventListener("click", hint);
  $("#refresh-inspection").addEventListener("click", () => inspect());
  $("#success-close").addEventListener("click", () =>
    $("#success-dialog").close(),
  );
  $("#copy-path").addEventListener(
    "click",
    () => state.active && copy(state.active.path, $("#copy-path")),
  );
  document
    .querySelectorAll(".copy-command")
    .forEach((button) =>
      button.addEventListener("click", () =>
        copy(
          $(button.dataset.command === "cd" ? "#command-cd" : "#command-merge")
            .textContent,
          button,
        ),
      ),
    );
  document.querySelectorAll("[data-filter]").forEach((button) =>
    button.addEventListener("click", () => {
      state.filter = button.dataset.filter;
      document.querySelectorAll("[data-filter]").forEach((item) => {
        item.classList.toggle("active", item === button);
        item.setAttribute("aria-pressed", String(item === button));
      });
      renderExercises();
    }),
  );
  $("#exercise-search").addEventListener("input", (event) => {
    state.query = event.target.value.trim().toLowerCase();
    renderExercises();
  });
  document.addEventListener("keydown", (event) => {
    if (
      event.key === "/" &&
      !["INPUT", "TEXTAREA"].includes(document.activeElement.tagName)
    ) {
      event.preventDefault();
      $("#exercise-search").focus();
    }
  });
}

init();
