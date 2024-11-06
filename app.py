import logging
import firebase_admin
import json
import re
from firebase_admin import exceptions, messaging
from flask import Flask, request, jsonify
from flask_httpauth import HTTPBasicAuth
from my_secrets import API_SECRET, FIREBASE_CONFIG, VAPID_PUBLIC_KEY # BADGE_ICON ToDo

app = Flask(__name__)
firebase_app = firebase_admin.initialize_app()

basic_auth = HTTPBasicAuth()

API_KEY = 'pfZOoRCW-kHtAHQcqiDvb_sy5o8Hj14J_ahUBQgPfOI'

USER_DEVICE_MAP = {'raven_erp-omniverse.com':{}, 'hrms_erp-omniverse.com':{}, 'crm_erp-omniverse.com':{}}

DECORATION = {
    'raven_erp-omniverse.com': {
        'new_message': {'pattern': r'.*?(New message|Message received).*?', 'template': '💬 {title}'},
        'mention': {'pattern': r'.*?(mentioned you|tagged you).*?', 'template': '👋 {title}'},
        'chat_invite': {'pattern': r'.*?(invited you to chat|new chat invitation).*?', 'template': '👥 {title}'}
    },
    'hrms_erp-omniverse.com': {
        'info': {'pattern': r'.*?(Info|Notice|Note).*?', 'template': 'ℹ️ {title}'},
        'leave_request': {'pattern': r'.*?(Leave Request|Time Off Request).*?', 'template': '🗓️ {title}'},
        'timesheet': {'pattern': r'.*?(Timesheet|Time Entry).*?', 'template': '⏰ {title}'},
        'payroll': {'pattern': r'.*?(Payroll|Salary|Payment).*?', 'template': '💰 {title}'},
        'attendance': {'pattern': r'.*?(Attendance|Check-in|Check-out).*?', 'template': '📋 {title}'}
    },
    'crm_erp-omniverse.com': {
        'new_lead': {'pattern': r'.*?(New Lead|Lead Created).*?', 'template': '💯💰 {title}'},
        'new_message': {'pattern': r'.*?(New Message|Message Received).*?', 'template': '📨 {title}'},
        'meeting': {'pattern': r'.*?(Meeting|Appointment|Schedule).*?', 'template': '📅 {title}'},
        'task': {'pattern': r'.*?(Task|Todo|Assignment).*?', 'template': '✔️ {title}'}
    }
}

ICONS = {
    'raven_erp-omniverse.com': 'icons/raven.png',
    'hrms_erp-omniverse.com': 'icons/hrms.png',
    'crm_erp-omniverse.com': 'icons/crm.png'
}

TOPICS = []

import os

def ensure_file_exists(filename, default_value):
    if not os.path.exists(filename):
        os.makedirs(os.path.dirname(filename), exist_ok=True)
        with open(filename, 'w') as jsonfile:
            json.dump(default_value, jsonfile)

def load_json_file(filename, default_value):
    ensure_file_exists(filename, default_value)
    with open(filename, 'r') as jsonfile:
        return json.load(jsonfile)

# Ensure files exist and load data
ensure_file_exists('user-device-map.json', USER_DEVICE_MAP)
ensure_file_exists('decoration.json', DECORATION)
ensure_file_exists('icons.json', ICONS)

USER_DEVICE_MAP = load_json_file('user-device-map.json', USER_DEVICE_MAP)
DECORATION = load_json_file('decoration.json', DECORATION)
ICONS = {k: v for k, v in load_json_file('icons.json', ICONS).items() if os.path.exists(v)}

def save_map_to_file(map):
	with open('user-device-map.json', 'w') as jsonfile:
		json.dump(map, jsonfile)
	print(json.dumps(map, indent=4))

@app.get('/api/method/notification_relay.api.get_config')
def get_config():
	response = {}
	response['vapid_public_key'] = VAPID_PUBLIC_KEY
	response['config'] = FIREBASE_CONFIG
	return response

@basic_auth.verify_password
def verify_api_key(api_key, api_secret):
	if api_key==API_KEY and api_secret==API_SECRET:
		return True
	return False

@app.post("/api/method/notification_relay.api.topic.subscribe")
@basic_auth.login_required
def subscribe_to_topic():
	project_name = request.args.get('project_name')
	site_name = request.args.get('site_name')
	key = f'{project_name}_{site_name}'
	user_id = request.args.get('user_id')
	topic_name = request.args.get('topic_name')
	app.logger.debug(f'Topic Subscribe Request - {request.args}')
	if user_id in USER_DEVICE_MAP.get(key):
		registration_tokens = USER_DEVICE_MAP.get(key).get(user_id)
		if registration_tokens:
			response = messaging.subscribe_to_topic(registration_tokens, topic_name)
			app.logger.info(f'{response.success_count} tokens were subscribed successfully')
			response = {
			'message':{
				'success':200,
				'message':'User subscribed'
				}
			}
			return response
		else:
			app.logger.info(f'No tokens found for user {user_id} to subscribe')
	app.logger.info(f'{user_id} not subscribed for push notifications')
	response = {
		'exc':{
			'status_code':404,
			'message':f'{user_id} not subscribed to push notifications'
			}
	}
	return response, 400

@app.post("/api/method/notification_relay.api.topic.unsubscribe")
@basic_auth.login_required
def unsubscribe_to_topic():
	project_name = request.args.get('project_name')
	site_name = request.args.get('site_name')
	key = f'{project_name}_{site_name}'
	user_id = request.args.get('user_id')
	topic_name = request.args.get('topic_name')
	app.logger.debug(f'Topic Unsubscribe Request - {request.args}')
	if user_id in USER_DEVICE_MAP.get(key):
		registration_tokens = USER_DEVICE_MAP.get(key).get(user_id)
		if registration_tokens:
			response = messaging.unsubscribe_from_topic(registration_tokens, topic_name)
			app.logger.info(f'{response.success_count} tokens were unsubscribed successfully')
			response = {
			'message':{
				'success':200,
				'message':f'User {user_id} unsubscribed from {topic_name} topic'
				}
			}
			return response
	response = {
		'exc':{
			'status_code':404,
			'message':f'{user_id} not subscribed to push notifications'
			}
	}
	return response, 400

@app.post("/api/method/notification_relay.api.token.add")
@basic_auth.login_required
def add_token():
	project_name = request.args.get('project_name')
	site_name = request.args.get('site_name')
	key = f'{project_name}_{site_name}'
	user_id = request.args.get('user_id')
	fcm_token = request.args.get('fcm_token')
	app.logger.debug(f'Add Token Request - {request.args}')
	if user_id in USER_DEVICE_MAP.get(key):
		if fcm_token in USER_DEVICE_MAP[key][user_id]:
			response = {
				'message':{
					'success':200,
					'message':'User Token duplicate found'
					}
			}
			app.logger.info(f'Duplicate token found for user {user_id}')
			return response
		else:
			USER_DEVICE_MAP[key][user_id].append(fcm_token)
			app.logger.info(f'Token added for user {user_id}')
	else:
		USER_DEVICE_MAP[key][user_id] = [fcm_token]
		app.logger.info(f'User entry created & Token added for user {user_id}')
	save_map_to_file(USER_DEVICE_MAP)
	response = {
		'message':{
			'success':200,
			'message':'User Token added'
			}
	}
	return response

@app.post("/api/method/notification_relay.api.token.remove")
@basic_auth.login_required
def remove_token():
	project_name = request.args.get('project_name')
	site_name = request.args.get('site_name')
	key = f'{project_name}_{site_name}'
	user_id = request.args.get('user_id')
	fcm_token = request.args.get('fcm_token')
	response = {
		'message':{
			'success':200,
			'message':'User Token removed'
			}
	}
	app.logger.debug(f'Remove Token Request - {request.args}')
	if user_id in USER_DEVICE_MAP.get(key) and USER_DEVICE_MAP.get(key).get(user_id):
		try:
			USER_DEVICE_MAP[key][user_id].remove(fcm_token)
			save_map_to_file(USER_DEVICE_MAP)
		except ValueError:
			app.logger.info(f'FCM Token not found for user {user_id}')
	return response

@app.post("/api/method/notification_relay.api.send_notification.user")
@basic_auth.login_required
def send_notification_to_user():
	project_name = request.args.get('project_name')
	site_name = request.args.get('site_name')
	key = f'{project_name}_{site_name}'
	user_id = request.args.get('user_id')
	title = request.args.get('title')
	body = request.args.get('body')
	data = json.loads(request.args.get('data'))
	notification_icon = data.get('notification_icon')
	if not notification_icon:
		notification_icon = ICONS.get(key, '')

	# Check title against decoration patterns if project exists
	if key in DECORATION:
		for notification_type, config in DECORATION[key].items():
			if re.match(config['pattern'], title):
				title = config['template'].format(title=title)
				break

	app.logger.debug(f'User Notification Request - {request.args}')
	registration_tokens = []
	if user_id in USER_DEVICE_MAP.get(key) and USER_DEVICE_MAP.get(key).get(user_id):
		registration_tokens = USER_DEVICE_MAP.get(key).get(user_id)
		message = messaging.MulticastMessage(
			webpush=messaging.WebpushConfig(
				notification=messaging.WebpushNotification(
					title=title,
					body=body,
					icon=notification_icon,
					#ToDo: Badge count server-side
					#badge=BADGE_ICON
				),
				fcm_options=messaging.WebpushFCMOptions(link=data.get('click_action')),
			),
			tokens=registration_tokens
		)
		try:
			response = messaging.send_each_for_multicast(message)
			if response.failure_count > 0:
				responses = response.responses
				failed_tokens = []
				for idx, resp in enumerate(responses):
					if not resp.success:
						# The order of responses corresponds to the order of the registration tokens.
						failed_tokens.append(registration_tokens[idx])
						if isinstance(resp.exception, exceptions.NotFoundError):
							USER_DEVICE_MAP.get(key).get(user_id).remove(registration_tokens[idx])
							save_map_to_file(USER_DEVICE_MAP)
						else:
							app.logger.debug(f'Exception : {resp.exception}, {resp.exception.code}, {resp.exception.cause}, {resp.exception.http_response}')
			app.logger.info(f'Successfully sent message: {response.success_count}, {[resp.message_id for resp in response.responses]}')
		except exceptions.FirebaseError:
			pass
		response = {
			'message':{
				'success':200,
				'message':f'{response.success_count} Notiifcation sent to {user_id} user'
				}
		}
	else:
		response = {
			'exc':{
				'status_code':404,
				'message':f'{user_id} not subscribed to push notifications'
				}
		}
		app.logger.info(f'User {user_id} has not enabled notifications')
		return response, 400
	return response

@app.post("/api/method/notification_relay.api.send_notification.topic")
@basic_auth.login_required
def send_notification_to_topic():
	topic = request.args.get('topic_name')
	title = request.args.get('title')
	body = request.args.get('body')
	data = json.loads(request.args.get('data'))
	notification_icon = data.get('notification_icon')
	if not notification_icon:
		notification_icon = ''
	app.logger.debug(f'Topic Notification Request - {request.args}')
	message = messaging.Message(
        webpush=messaging.WebpushConfig(
            notification=messaging.WebpushNotification(
                title=title,
                body=body,
                icon=notification_icon,
				badge= BADGE_ICON
            ),
			fcm_options=messaging.WebpushFCMOptions(link=data.get('click_action')),
        ),
        topic=topic,
    )
	response = messaging.send(message)
	app.logger.info(f'Successfully sent message: {response}')
	response = {
		'message':{
			'success':200,
			'message':f'Notiifcation sent to {topic} topic'
			}
	}
	return response

if __name__ != '__main__':
    gunicorn_logger = logging.getLogger('gunicorn.error')
    app.logger.handlers = gunicorn_logger.handlers
    app.logger.setLevel(gunicorn_logger.level)
