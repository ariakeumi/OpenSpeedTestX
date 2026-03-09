(function() {
  var config = window.ostHistoryConfig || {};
  var endpoint = config.endpoint || "/api/history";
  var historyLimit = config.limit || 20;
  var emptyNode;
  var tableBodyNode;
  var detectedClientIP = "";
  var clientIPState = "idle";
  var clientIPCallbacks = [];

  function historyReady(callback) {
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", callback);
      return;
    }
    callback();
  }

  function numberOrZero(value, digits) {
    var parsed = Number(value);
    if (!isFinite(parsed)) {
      parsed = 0;
    }
    return parsed.toFixed(digits);
  }

  function formatTime(value) {
    var date = new Date(value);
    if (isNaN(date.getTime())) {
      return "--";
    }
    return date.toLocaleString();
  }

  function setEmptyMessage(message) {
    if (!emptyNode) {
      return;
    }
    emptyNode.textContent = message;
  }

  function isLoopbackIP(ip) {
    return ip === "127.0.0.1" || ip === "::1" || ip === "0.0.0.0" || ip === "::";
  }

  function isPrivateIPv4(ip) {
    return /^10\./.test(ip) || /^192\.168\./.test(ip) || /^172\.(1[6-9]|2[0-9]|3[0-1])\./.test(ip);
  }

  function isValidIPv4(ip) {
    var parts = ip.split(".");
    if (parts.length !== 4) {
      return false;
    }
    return parts.every(function(part) {
      if (!/^\d{1,3}$/.test(part)) {
        return false;
      }
      var value = Number(part);
      return value >= 0 && value <= 255;
    });
  }

  function isValidIPv6(ip) {
    var parts;
    var compactMatches;
    var filledParts = 0;
    var i;

    if (ip.indexOf(":") === -1 || ip.indexOf(":::") !== -1) {
      return false;
    }

    compactMatches = ip.match(/::/g);
    if (compactMatches && compactMatches.length > 1) {
      return false;
    }

    parts = ip.split(":");
    if (parts.length > 8 + (compactMatches ? 1 : 0)) {
      return false;
    }

    for (i = 0; i < parts.length; i++) {
      if (parts[i] === "") {
        continue;
      }
      if (!/^[0-9a-fA-F]{1,4}$/.test(parts[i])) {
        return false;
      }
      filledParts++;
    }

    if (compactMatches) {
      return filledParts < 8;
    }

    return parts.length === 8 && filledParts === 8;
  }

  function isValidIPAddress(ip) {
    return isValidIPv4(ip) || isValidIPv6(ip);
  }

  function extractCandidateIPs(text, bucket) {
    var matches = text ? text.match(/\b(?:\d{1,3}\.){3}\d{1,3}\b|\b[a-f0-9:]+:+[a-f0-9:]+\b/gi) : null;
    if (!matches) {
      return;
    }
    matches.forEach(function(match) {
      var ip = String(match).trim();
      if (!ip || !isValidIPAddress(ip) || isLoopbackIP(ip)) {
        return;
      }
      if (bucket.indexOf(ip) === -1) {
        bucket.push(ip);
      }
    });
  }

  function chooseClientIP(candidates) {
    var i;
    for (i = 0; i < candidates.length; i++) {
      if (isPrivateIPv4(candidates[i])) {
        return candidates[i];
      }
    }
    return candidates.length > 0 ? candidates[0] : "";
  }

  function flushClientIPCallbacks(value) {
    var callbacks = clientIPCallbacks.slice();
    clientIPCallbacks = [];
    callbacks.forEach(function(callback) {
      callback(value);
    });
  }

  function finishClientIPDetection(value) {
    if (clientIPState === "done") {
      return;
    }
    detectedClientIP = value || "";
    clientIPState = "done";
    flushClientIPCallbacks(detectedClientIP);
  }

  function startClientIPDetection() {
    var PeerConnection = window.RTCPeerConnection || window.webkitRTCPeerConnection || window.mozRTCPeerConnection;
    var pc;
    var candidates = [];
    var timeoutId;
    var completed = false;

    clientIPState = "pending";

    if (!PeerConnection) {
      finishClientIPDetection("");
      return;
    }

    function cleanup() {
      if (timeoutId) {
        clearTimeout(timeoutId);
      }
      if (pc && pc.close) {
        pc.close();
      }
    }

    function complete() {
      if (completed) {
        return;
      }
      completed = true;
      cleanup();
      finishClientIPDetection(chooseClientIP(candidates));
    }

    function onOfferReady(desc) {
      extractCandidateIPs(desc && desc.sdp ? desc.sdp : "", candidates);
      var setResult;
      try {
        setResult = pc.setLocalDescription(desc);
        if (setResult && typeof setResult.then === "function") {
          setResult["catch"](complete);
        }
      } catch (error) {
        complete();
      }
    }

    function onOfferError() {
      complete();
    }

    try {
      pc = new PeerConnection({iceServers: []});
      if (pc.createDataChannel) {
        pc.createDataChannel("ost-history");
      }
      pc.onicecandidate = function(event) {
        if (event && event.candidate && event.candidate.candidate) {
          extractCandidateIPs(event.candidate.candidate, candidates);
          return;
        }
        complete();
      };
      timeoutId = setTimeout(complete, 1500);

      var offerResult = pc.createOffer(onOfferReady, onOfferError);
      if (offerResult && typeof offerResult.then === "function") {
        offerResult.then(onOfferReady)["catch"](onOfferError);
      }
    } catch (error) {
      complete();
    }
  }

  function withClientIP(callback) {
    if (clientIPState === "done") {
      callback(detectedClientIP);
      return;
    }
    clientIPCallbacks.push(callback);
    if (clientIPState === "idle") {
      startClientIPDetection();
    }
  }

  function renderHistory(entries) {
    tableBodyNode.innerHTML = "";

    if (!entries || entries.length === 0) {
      setEmptyMessage("还没有测速记录，完成一次测速后会自动出现在这里。");
      emptyNode.style.display = "block";
      return;
    }

    emptyNode.style.display = "none";

    entries.forEach(function(entry) {
      var row = document.createElement("tr");
      var cells = [
        entry.clientIp || "--",
        numberOrZero(entry.downloadMbps, 2) + " Mbps",
        numberOrZero(entry.uploadMbps, 2) + " Mbps",
        numberOrZero(entry.pingMs, 1) + " ms",
        numberOrZero(entry.jitterMs, 1) + " ms",
        formatTime(entry.createdAt)
      ];

      cells.forEach(function(value) {
        var cell = document.createElement("td");
        cell.textContent = value;
        row.appendChild(cell);
      });

      var actionCell = document.createElement("td");
      var deleteButton = document.createElement("button");
      deleteButton.className = "history-row-delete";
      deleteButton.type = "button";
      deleteButton.textContent = "删除";
      deleteButton.onclick = function() {
        deleteHistoryEntry(entry.id);
      };
      actionCell.appendChild(deleteButton);
      row.appendChild(actionCell);

      tableBodyNode.appendChild(row);
    });
  }

  function requestJSON(url, options, onSuccess, onError) {
    var xhr = new XMLHttpRequest();
    var method = options.method || "GET";
    var headers = options.headers || {};

    xhr.open(method, url, true);
    Object.keys(headers).forEach(function(key) {
      xhr.setRequestHeader(key, headers[key]);
    });

    xhr.onreadystatechange = function() {
      if (xhr.readyState !== 4) {
        return;
      }
      if (xhr.status >= 200 && xhr.status < 300) {
        if (xhr.status === 204 || !xhr.responseText) {
          onSuccess(null);
          return;
        }
        try {
          onSuccess(JSON.parse(xhr.responseText));
        } catch (error) {
          onError(error);
        }
        return;
      }
      onError(new Error(xhr.responseText || ("Request failed: " + xhr.status)));
    };

    xhr.onerror = function() {
      onError(new Error("Network request failed"));
    };

    xhr.send(options.body || null);
  }

  function loadHistory() {
    requestJSON(endpoint + "?limit=" + encodeURIComponent(historyLimit), {
      method: "GET",
      headers: {
        "Accept": "application/json"
      }
    }, function(entries) {
      renderHistory(entries || []);
    }, function(error) {
      renderHistory([]);
      setEmptyMessage("历史记录加载失败: " + error.message);
      emptyNode.style.display = "block";
    });
  }

  function persistResult(detail) {
    withClientIP(function(clientIP) {
      var payload = {};
      var key;
      for (key in detail) {
        if (Object.prototype.hasOwnProperty.call(detail, key)) {
          payload[key] = detail[key];
        }
      }
      if (clientIP) {
        payload.clientIp = clientIP;
      }

      requestJSON(endpoint, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "Accept": "application/json"
        },
        body: JSON.stringify(payload)
      }, function() {
        loadHistory();
      }, function(error) {
        if (window.console && console.warn) {
          console.warn("History save failed", error);
        }
      });
    });
  }

  function deleteHistoryEntry(id) {
    if (!id) {
      return;
    }
    if (!window.confirm("确定要删除这条测速记录吗？")) {
      return;
    }
    requestJSON(endpoint + "?id=" + encodeURIComponent(id), {
      method: "DELETE",
      headers: {
        "Accept": "application/json"
      }
    }, function() {
      loadHistory();
    }, function(error) {
      setEmptyMessage("历史记录删除失败: " + error.message);
      emptyNode.style.display = "block";
    });
  }

  historyReady(function() {
    emptyNode = document.getElementById("historyEmpty");
    tableBodyNode = document.getElementById("historyTableBody");

    if (!tableBodyNode) {
      return;
    }
    startClientIPDetection();
    window.addEventListener("ost:result", function(event) {
      if (event && event.detail) {
        persistResult(event.detail);
      }
    });

    loadHistory();
  });
})();
