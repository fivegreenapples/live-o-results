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
				console.log("UNhandled websocket event: "+msgData.Name, msgData.Data)
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
		}
		return service

	}
])


App.controller("mainCtrl", [
    "$scope",
	"$http",
	"$timeout",
	"socket",
    function($scope, $http, $timeout, Socket) {
		$scope.socketStatus = {
			showBadness: false,
			connected: false
		}
		$scope.results = {
			Title: "NOT LOADED"
		}

		Socket.addListener("open", function() {
			$scope.socketStatus.connected = true
		}, $scope)
		Socket.addListener("close", function() {
			$scope.socketStatus.connected = false
		}, $scope)
		$timeout(function() {
			$scope.socketStatus.showBadness = true
		}, 500)
		Socket.addEventListener("newResults", function(resultSet) {
			console.log(resultSet)
			processResultSet(angular.copy(resultSet))
		}, $scope)
		Socket.connect()


		function processResultSet(resultSet) {
			resultSet.Results.Courses.forEach(function(course, i) {
				course.Competitors.forEach(function(competitor) {
					// Time arrives in nanoseconds!
					var timeTotalSeconds = Math.floor(competitor.Time / 1000000000)
					var timeMins = Math.floor(timeTotalSeconds / 60)
					var timeSeconds = timeTotalSeconds % 60
					if (timeSeconds <= 9) {
						timeSeconds = "0"+timeSeconds
					}
					competitor.Time = timeMins+":"+timeSeconds
				})
			})
			$scope.results = resultSet.Results
		}


	}
])



