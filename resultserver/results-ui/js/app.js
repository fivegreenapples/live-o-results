App = angular.module('rwm', [])

App.service("socket", [
	"$timeout",
	"$q",
	function($timeout, $q) {

		var sock
		var connectInitiated = false
		var socketOpen = false
		var connectAttemptNumber = 0

		var handlers = {
			open: [],
			close: [],
			error: [],
			message: [],
		}
		var evhandlers = {}

		var apiRequests = {}
		var apiTag = 0

		function msgHandler(msg) {
			var msgData
			try {
				msgData = JSON.parse(msg)
			} catch(err) {}
			if (!msgData || !msgData.Type) {
				console.error("Bad websocket message format: "+msg)
				return
			}
			if (msgData.Type == "Error") {
				console.error("Websocket error: "+msgData.Msg)
				return
			}
			if (msgData.Type == "API") {
				return apiMsgHandler(msgData.Msg)
			}
			if (msgData.Type == "Event") {
				return eventMsgHandler(msgData.Msg)
			}
		}
		function apiMsgHandler(msgData) {
			if (!msgData.Resp) {
				console.error("No Data packet for an API response.", msgData)
				return
			}
			if (!apiRequests[msgData.Tag]) {
				console.error("Unknown API response received with tag: "+msgData.Tag, msgData)
				return
			}
			var response = msgData.Resp
			if (!response.Success) {
				apiRequests[msgData.Tag].reject(response.ErrorMessage)
				return
			}
			apiRequests[msgData.Tag].resolve(response.Result)
		}
		function eventMsgHandler(msgData) {
			if (!msgData.Name) {
				console.error("No name for a socket event.", msgData)
				return
			}
			if (evhandlers[msgData.Name]) {
				evhandlers[msgData.Name].forEach(function(h) {
					if (h) h(msgData.Data)
				})
			} else {
				console.log("Unhandled websocket event: "+msgData.Name, msgData.Data)
			}
		}
		handlers.message.push(msgHandler)

		function attemptConnect() {
			var timeout
			if (connectAttemptNumber == 0) {
				timeout = 0
			} else if (connectAttemptNumber<5) {
				timeout = 100
			} else if (connectAttemptNumber<10) {
				timeout = 1000
			} else if (connectAttemptNumber<13) {
				timeout = 3000
			} else {
				timeout = 5000
			}
			$timeout(function() {
				sock = new SockJS('/sockjs/')
				sock.addEventListener('open', function (ev) {
					if (connectAttemptNumber == 0) {
						console.log("Connected to websocket server.")
					} else {
						console.log("Reconnected to websocket server.")
					}
					socketOpen = true
					connectAttemptNumber = 0
					handlers.open.forEach(function(h) {
						if (h) h()
					})
				});
				sock.addEventListener('close', function (ev) {
					if (connectAttemptNumber == 0) {
						console.error("Connection to websocket has been lost.")
					}
					socketOpen = false
					connectAttemptNumber += 1
					attemptConnect()
					handlers.close.forEach(function(h) {
						if (h) h(ev)
					})
				});
				sock.addEventListener('error', function(ev) {
					console.log("socket error", ev);
					handlers.error.forEach(function(h) {
						if (h) h()
					})
				});
				sock.addEventListener('message', function(ev) {
					handlers.message.forEach(function(h) {
						if (h) h(ev.data)
					})
				});
			}, timeout)
		}
		
		function sendMessage(msg) {
			if (!socketOpen) {
				return false
			}
			sock.send(msg)
			return true
		}
		
		var service = {
			addListener: function(ev, handler, scope) {
				if (!(ev in handlers)) {
					console.error("Listener '"+ev+"' not supported by socket service.")
					return
				}
				var handlerId = handlers[ev].length
				handlers[ev].push(function() {
					scope.$apply(Function.prototype.apply.bind(handler, null, arguments))
				})
				scope.$on("$destroy", function() {
					handlers[ev][handlerId] = null
				})
			}
			,addEventListener: function(ev, handler, scope) {
				if (!evhandlers[ev]) evhandlers[ev] = []
				var handlerId = evhandlers[ev].length
				evhandlers[ev].push(function() {
					scope.$apply(Function.prototype.apply.bind(handler, null, arguments))
				})
				scope.$on("$destroy", function() {
					evhandlers[ev][handlerId] = null
				})
			}
			,connect: function() {
				if (connectInitiated) return
				attemptConnect()
			}
			,sendAPIRequest: function(action, params) {
				var tag = ++apiTag
				var msg = JSON.stringify({
					Type: "API",
					Msg: {
						Tag: tag,
						Req: {
							Action: action,
							Params: params
						}
					}
				})
				apiRequests[tag] = $q.defer()
				var ok = sendMessage(msg)
				if (!ok) {
					apiRequests[tag].reject("Websocket not connected")
				}
				return apiRequests[tag].promise
			}
			,sendRawMessage: function(msg) {
				return sendMessage(msg)
			}
		}
		return service

	}
])

App.service("results", [function() {

	var service = {
		addDelta: function(hash, results, delta) {

			if (hash != delta.Old) {
				console.log("Delta.Old didn't match current result hash.", delta.Old, hash)
				return false
			}

			// clone current result set
			var newResultSet = {
				Hash:    delta.New,
				Results: angular.copy(results)
			}

			// copy over updated details
			if (delta.hasOwnProperty("Title") && delta.Title !== null) {
				newResultSet.Results.Title = delta.Title
			}

			if (delta.hasOwnProperty("Courses") && delta.Courses !== null) {
				var cursorA = 0, cursorB = 0
				var toAdd = Object.keys(delta.Courses.Added).length
				newResultSet.Results.Courses = []
				while (toAdd > 0 || (results.Courses && cursorA < results.Courses.length)) {
					if (toAdd > 0 && delta.Courses.Added[cursorB]) {
						newResultSet.Results.Courses.push(delta.Courses.Added[cursorB])
						toAdd--
						cursorB++
						continue
					}

					if (results.Courses && cursorA < results.Courses.length) {
						if (!delta.Courses.Removed.hasOwnProperty(cursorA)) {
							newResultSet.Results.Courses.push(results.Courses[cursorA])
						}
					}

					cursorA++
					cursorB++
				}
			}

			if (delta.hasOwnProperty("Competitors") && delta.Competitors !== null) {
				
				angular.forEach(delta.Competitors, function(compDelta, courseIndex) {
					var cursorA = 0, cursorB = 0
					var toAdd = Object.keys(compDelta.Added).length
					var oldSet = newResultSet.Results.Courses[courseIndex].Competitors
					var newSet = []
					while (toAdd > 0 || cursorA < oldSet.length) {
						if (toAdd > 0 && compDelta.Added[cursorB]) {
							newSet.push(compDelta.Added[cursorB])
							toAdd--
							cursorB++
							continue
						}

						if (cursorA < oldSet.length) {
							if (!compDelta.Removed.hasOwnProperty(cursorA)) {
								newSet.push(oldSet[cursorA])
							}
						}

						cursorA++
						cursorB++
					}
					newResultSet.Results.Courses[courseIndex].Competitors = newSet
				})
			}
							
			return newResultSet

		}
	}
	return service

}])
App.controller("mainCtrl", [
    "$scope",
	"$http",
	"$timeout",
	"socket",
	"results",
    function($scope, $http, $timeout, Socket, Results) {
		$scope.socketStatus = {
			showError: false,
			connected: false
		}
		$scope.resultsHash = 0
		$scope.results = {
			Title: "Event Title, Event Date"
		}

		$scope.courseVisibility = {}
		loadCourseVisibility()

		Socket.addListener("open", function() {
			$scope.socketStatus.connected = true
			Socket.sendRawMessage("RequestResults")
		}, $scope)
		Socket.addListener("close", function() {
			$scope.socketStatus.connected = false
		}, $scope)
		$timeout(function() {
			$scope.socketStatus.showError = true
		}, 500)
		Socket.addEventListener("NewResults", function(resultSet) {
			console.log("NewResults", resultSet)
			processResultSet(angular.copy(resultSet))
		}, $scope)
		Socket.addEventListener("NewDelta", function(resultDelta) {
			console.log("NewDelta", resultDelta)
			var newResultSet = Results.addDelta($scope.resultsHash, $scope.results, resultDelta)
			if (!newResultSet) {
				// need to request a full submission
				console.log("Requesting new results")
				Socket.sendRawMessage("RequestResults")
				return
			}
			console.log("Calculated Results", newResultSet)
			processResultSet(newResultSet)
		}, $scope)
		Socket.connect()


		function processResultSet(resultSet) {
			if (!resultSet || !resultSet.Results || !resultSet.Results.Title) return

			if (resultSet.Results.Courses) {
				resultSet.Results.Courses.forEach(function(course, i) {
					if (!(course.Title in $scope.courseVisibility)) {
						$scope.courseVisibility[course.Title] = true
					}
					if (!course.Competitors) return 
					course.Competitors.forEach(function(competitor) {
						// Time arrives in nanoseconds!
						var timeTotalSeconds = Math.floor(competitor.Time / 1000000000)
						var timeMins = Math.floor(timeTotalSeconds / 60)
						var timeSeconds = timeTotalSeconds % 60
						if (timeSeconds <= 9) {
							timeSeconds = "0"+timeSeconds
						}
						competitor.TimeFormatted = timeMins+":"+timeSeconds
					})
				})
			}
			$scope.resultsHash = resultSet.Hash
			$scope.results     = resultSet.Results
			$scope.storeCourseVisibility()
		}

		$scope.toggleAll = function(val) {
			for (k in $scope.courseVisibility) {
				$scope.courseVisibility[k] = val
			}
			$scope.storeCourseVisibility()
		}
		 $scope.storeCourseVisibility = function() {
			window.localStorage.setItem("cv", JSON.stringify($scope.courseVisibility))
		}
		function loadCourseVisibility() {
			var cv = window.localStorage.getItem("cv")
			if (cv) {
				var cvObj = JSON.parse(cv)
				$scope.courseVisibility = cvObj
			}
		}
	}
])



