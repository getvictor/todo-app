// Use relative URL so it works with whatever port the server is running on
const API_URL = '';

let tasks = [];

async function fetchTasks() {
    try {
        const response = await fetch(`${API_URL}/tasks`);
        if (!response.ok) {
            throw new Error('Failed to fetch tasks');
        }
        tasks = await response.json();
        renderTasks();
    } catch (error) {
        console.error('Error fetching tasks:', error);
        alert('Failed to fetch tasks. Make sure the backend server is running.');
    }
}

async function createTask(title) {
    try {
        const response = await fetch(`${API_URL}/tasks`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ title }),
        });

        if (!response.ok) {
            throw new Error('Failed to create task');
        }

        const newTask = await response.json();
        tasks.unshift(newTask);
        renderTasks();
        return newTask;
    } catch (error) {
        console.error('Error creating task:', error);
        alert('Failed to create task');
    }
}

async function deleteTask(id) {
    try {
        const response = await fetch(`${API_URL}/tasks/${id}`, {
            method: 'DELETE',
        });

        if (!response.ok) {
            throw new Error('Failed to delete task');
        }

        tasks = tasks.filter(task => task.id !== id);
        renderTasks();
    } catch (error) {
        console.error('Error deleting task:', error);
        alert('Failed to delete task');
    }
}

async function completeTask(id) {
    try {
        const response = await fetch(`${API_URL}/tasks/${id}/complete`, {
            method: 'POST',
        });

        if (!response.ok) {
            throw new Error('Failed to complete task');
        }

        const updatedTask = await response.json();
        const taskIndex = tasks.findIndex(task => task.id === id);
        if (taskIndex !== -1) {
            tasks[taskIndex] = updatedTask;
            renderTasks();
        }
    } catch (error) {
        console.error('Error completing task:', error);
        alert('Failed to complete task');
    }
}

function renderTasks() {
    const taskList = document.getElementById('taskList');
    taskList.innerHTML = '';

    if (tasks.length === 0) {
        taskList.innerHTML = '<li class="empty-state">No tasks yet. Add one above!</li>';
        return;
    }

    tasks.forEach(task => {
        const li = document.createElement('li');
        li.className = 'task-item';
        if (task.completed) {
            li.classList.add('completed');
        }

        const taskContent = document.createElement('div');
        taskContent.className = 'task-content';

        const checkbox = document.createElement('input');
        checkbox.type = 'checkbox';
        checkbox.checked = task.completed;
        checkbox.disabled = task.completed;
        checkbox.addEventListener('change', () => {
            if (!task.completed) {
                completeTask(task.id);
            }
        });

        const title = document.createElement('span');
        title.className = 'task-title';
        title.textContent = task.title;

        taskContent.appendChild(checkbox);
        taskContent.appendChild(title);

        const deleteButton = document.createElement('button');
        deleteButton.className = 'delete-button';
        deleteButton.textContent = 'Delete';
        deleteButton.addEventListener('click', () => deleteTask(task.id));

        li.appendChild(taskContent);
        li.appendChild(deleteButton);
        taskList.appendChild(li);
    });
}

document.addEventListener('DOMContentLoaded', () => {
    const taskInput = document.getElementById('taskInput');
    const addButton = document.getElementById('addButton');

    async function handleAddTask() {
        const title = taskInput.value.trim();
        if (title) {
            await createTask(title);
            taskInput.value = '';
            taskInput.focus();
        }
    }

    addButton.addEventListener('click', handleAddTask);
    taskInput.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            handleAddTask();
        }
    });

    fetchTasks();
});